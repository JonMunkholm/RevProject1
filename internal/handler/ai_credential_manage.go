package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/config"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

// DeleteProviderCredential removes a credential by identifier.
func (h *AI) DeleteProviderCredential(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.CredentialStore == nil {
		err := errors.New("credential store not configured")
		if respondWithAINotice(w, r, "error", "Credentials unavailable", err) {
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "credentials unavailable", err)
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		err := errors.New("session missing")
		if respondWithAINotice(w, r, "error", "Authentication required", err) {
			return
		}
		RespondWithError(w, http.StatusUnauthorized, "authentication required", err)
		return
	}

	credentialID, err := uuid.Parse(chi.URLParam(r, "credentialID"))
	if err != nil {
		if respondWithAINotice(w, r, "error", "Invalid credential id", err) {
			return
		}
		RespondWithError(w, http.StatusBadRequest, "invalid credential id", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), config.DefaultHandlerTimeout)
	defer cancel()

	record, err := h.CredentialStore.GetCredential(ctx, credentialID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if respondWithAINotice(w, r, "error", "Credential not found", err) {
				return
			}
			RespondWithError(w, http.StatusNotFound, "credential not found", err)
			return
		}
		if respondWithAINotice(w, r, "error", "Failed to load credential", err) {
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to load credential", err)
		return
	}

	if record.CompanyID != session.CompanyID {
		scopeErr := errors.New("credential scope mismatch")
		if respondWithAINotice(w, r, "error", "Credential not found", scopeErr) {
			return
		}
		RespondWithError(w, http.StatusNotFound, "credential not found", scopeErr)
		return
	}

	if ok, msg := canManageCredentialScope(session, record.UserID); !ok {
		permErr := errors.New("insufficient credential permissions")
		if respondWithAINotice(w, r, "error", msg, permErr) {
			return
		}
		RespondWithError(w, http.StatusForbidden, "insufficient permissions", permErr)
		return
	}

	if err := h.CredentialStore.DeleteCredential(ctx, credentialID); err != nil {
		if respondWithAINotice(w, r, "error", "Failed to delete credential", err) {
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to delete credential", err)
		return
	}

	eventMeta := map[string]any{
		"credential_id": credentialID.String(),
		"fingerprint":   record.Fingerprint,
		"scope":         scopeFromUserID(record.UserID),
		"is_default":    record.IsDefault,
	}
	if record.Label != nil && *record.Label != "" {
		eventMeta["label"] = *record.Label
	}
	if record.UserID.Valid {
		eventMeta["user_id"] = record.UserID.UUID.String()
	}
	h.recordCredentialEvent(ctx, session.CompanyID, record.UserID, session.UserID, record.ProviderID, "delete", eventMeta)

	triggerCredentialRefresh(w)
	if respondWithAINotice(w, r, "success", "Credential deleted", nil) {
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestProviderCredential validates a provider credential without persisting it.
func (h *AI) TestProviderCredential(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		err := errors.New("ai handler not configured")
		if respondWithAINotice(w, r, "error", "AI system unavailable", err) {
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "ai system unavailable", err)
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		err := errors.New("session missing")
		if respondWithAINotice(w, r, "error", "Authentication required", err) {
			return
		}
		RespondWithError(w, http.StatusUnauthorized, "authentication required", err)
		return
	}

	req, err := parseUpsertProviderCredentialRequest(r)
	if err != nil {
		if respondWithAINotice(w, r, "error", "Invalid payload", err) {
			return
		}
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		return
	}
	providerCandidate := strings.TrimSpace(chi.URLParam(r, "providerID"))
	if providerCandidate == "" {
		providerCandidate = req.Provider
	}
	providerID, _, err := h.normalizeProvider(r.Context(), providerCandidate)
	if err != nil {
		if respondWithAINotice(w, r, "error", "Unknown provider", err) {
			return
		}
		RespondWithError(w, http.StatusBadRequest, "unknown provider", err)
		return
	}

	scopeUser := uuid.NullUUID{UUID: session.UserID, Valid: true}
	scopeLabel := "user"
	if !(req.CredentialID != "" && req.APIKey == "") {
		if resolvedUser, resolvedScope, err := resolveCredentialScope(session, req.Scope, req.UserID); err == nil {
			scopeUser = resolvedUser
			scopeLabel = resolvedScope
		} else {
			h.handleTestFailure(w, r, session, providerID, scopeUser, scopeLabel, http.StatusBadRequest, "invalid scope", err)
			return
		}
	}

	var stored *ai.CredentialRecord
	if req.CredentialID != "" && req.APIKey == "" {
		credentialID, err := uuid.Parse(req.CredentialID)
		if err != nil {
			h.handleTestFailure(w, r, session, providerID, scopeUser, scopeLabel, http.StatusBadRequest, "invalid credential id", err)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), config.DefaultHandlerTimeout)
		defer cancel()

		record, err := h.CredentialStore.GetCredential(ctx, credentialID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				h.handleTestFailure(w, r, session, providerID, scopeUser, scopeLabel, http.StatusNotFound, "credential not found", err)
				return
			}
			h.handleTestFailure(w, r, session, providerID, scopeUser, scopeLabel, http.StatusInternalServerError, "failed to load credential", err)
			return
		}
		if record.CompanyID != session.CompanyID {
			h.handleTestFailure(w, r, session, providerID, scopeUser, scopeLabel, http.StatusNotFound, "credential not found", errors.New("credential scope mismatch"))
			return
		}
		stored = &record
		providerID = record.ProviderID
		scopeUser = record.UserID
		scopeLabel = scopeFromUserID(record.UserID)

		if ok, msg := canManageCredentialScope(session, scopeUser); !ok {
			permErr := errors.New("insufficient credential permissions")
			h.handleTestFailure(w, r, session, providerID, scopeUser, scopeLabel, http.StatusForbidden, msg, permErr)
			return
		}
	}

	if ok, msg := canManageCredentialScope(session, scopeUser); !ok {
		permErr := errors.New("insufficient credential permissions")
		h.handleTestFailure(w, r, session, providerID, scopeUser, scopeLabel, http.StatusForbidden, msg, permErr)
		return
	}

	if stored == nil {
		if req.APIKey == "" {
			h.handleTestFailure(w, r, session, providerID, scopeUser, scopeLabel, http.StatusBadRequest, "apiKey is required", errors.New("missing api key"))
			return
		}
		if err := validateAPIKeyFormat(providerID, req.APIKey); err != nil {
			h.handleTestFailure(w, r, session, providerID, scopeUser, scopeLabel, http.StatusBadRequest, "invalid api key", err)
			return
		}
	}

	meta := map[string]any{
		"status":       "success",
		"scope":        scopeLabel,
		"api_supplied": stored == nil,
	}
	if stored != nil {
		meta["credential_id"] = stored.ID.String()
		meta["fingerprint"] = stored.Fingerprint
		if stored.UserID.Valid {
			meta["user_id"] = stored.UserID.UUID.String()
		}
	}
	h.recordCredentialEvent(r.Context(), session.CompanyID, scopeUser, session.UserID, providerID, "test", meta)

	triggerCredentialRefresh(w)
	if respondWithAINotice(w, r, "success", "Credential test succeeded", nil) {
		return
	}

	RespondWithJSON(w, http.StatusOK, map[string]any{"ok": true, "provider": providerID, "companyId": session.CompanyID.String()})
}

func (h *AI) handleTestFailure(w http.ResponseWriter, r *http.Request, session auth.Session, providerID string, scopeUser uuid.NullUUID, scopeLabel string, status int, message string, err error) {
	if providerID == "" {
		providerID = h.DefaultProvider
	}
	if h.CredentialMetrics != nil {
		h.CredentialMetrics.CredentialTestFailure(session.CompanyID, providerID)
	}
	meta := map[string]any{
		"status": "failure",
		"scope":  scopeLabel,
	}
	if message != "" {
		meta["reason"] = message
	}
	if err != nil {
		meta["error"] = err.Error()
	}
	h.recordCredentialEvent(r.Context(), session.CompanyID, scopeUser, session.UserID, providerID, "test", meta)
	if respondWithAINotice(w, r, "error", message, err) {
		return
	}
	RespondWithError(w, status, message, err)
}

// UpsertProviderCredential stores or replaces a provider credential for the current company/user.
func (h *AI) UpsertProviderCredential(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.CredentialStore == nil || h.CredentialCipher == nil {
		err := errors.New("credential store not configured")
		if respondWithAINotice(w, r, "error", "Credentials unavailable", err) {
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "credentials unavailable", err)
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		err := errors.New("session missing")
		if respondWithAINotice(w, r, "error", "Authentication required", err) {
			return
		}
		RespondWithError(w, http.StatusUnauthorized, "authentication required", err)
		return
	}

	req, err := parseUpsertProviderCredentialRequest(r)
	if err != nil {
		if respondWithAINotice(w, r, "error", "Invalid payload", err) {
			return
		}
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		return
	}
	providerCandidate := chi.URLParam(r, "providerID")
	if providerCandidate == "" {
		providerCandidate = req.Provider
	}
	providerID, _, err := h.normalizeProvider(r.Context(), providerCandidate)
	if err != nil {
		if respondWithAINotice(w, r, "error", "Unknown provider", err) {
			return
		}
		RespondWithError(w, http.StatusBadRequest, "unknown provider", err)
		return
	}
	req.Provider = providerID

	if req.APIKey != "" {
		if err := validateAPIKeyFormat(providerID, req.APIKey); err != nil {
			if respondWithAINotice(w, r, "error", "Invalid API key", err) {
				return
			}
			RespondWithError(w, http.StatusBadRequest, "invalid api key", err)
			return
		}
	}

	scopeUser, _, err := resolveCredentialScope(session, req.Scope, req.UserID)
	if err != nil {
		if respondWithAINotice(w, r, "error", "Invalid scope", err) {
			return
		}
		RespondWithError(w, http.StatusBadRequest, "invalid scope", err)
		return
	}

	metadata := cloneMetadata(req.Metadata)
	if req.Model != "" {
		metadata["model"] = req.Model
	}
	if req.BaseURL != "" {
		metadata["base_url"] = req.BaseURL
	}

	if ok, msg := canManageCredentialScope(session, scopeUser); !ok {
		permErr := errors.New("insufficient credential permissions")
		if respondWithAINotice(w, r, "error", msg, permErr) {
			return
		}
		RespondWithError(w, http.StatusForbidden, "insufficient permissions", permErr)
		return
	}

	labelValue := strings.TrimSpace(req.Label)
	var labelPtr *string
	if labelValue != "" {
		labelPtr = &labelValue
	}

	ctx, cancel := context.WithTimeout(r.Context(), config.DefaultHandlerTimeout)
	defer cancel()

	var existing ai.CredentialRecord
	var hasExisting bool

	status := http.StatusCreated
	if req.CredentialID != "" {
		credentialID, err := uuid.Parse(req.CredentialID)
		if err != nil {
			if respondWithAINotice(w, r, "error", "Invalid credential id", err) {
				return
			}
			RespondWithError(w, http.StatusBadRequest, "invalid credential id", err)
			return
		}

		existing, err = h.CredentialStore.GetCredential(ctx, credentialID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if respondWithAINotice(w, r, "error", "Credential not found", err) {
					return
				}
				RespondWithError(w, http.StatusNotFound, "credential not found", err)
				return
			}
			if respondWithAINotice(w, r, "error", "Failed to load credential", err) {
				return
			}
			RespondWithError(w, http.StatusInternalServerError, "failed to load credential", err)
			return
		}

		if existing.CompanyID != session.CompanyID || existing.ProviderID != providerID {
			scopeErr := errors.New("credential does not belong to company")
			if respondWithAINotice(w, r, "error", "Credential scope mismatch", scopeErr) {
				return
			}
			RespondWithError(w, http.StatusForbidden, "credential scope mismatch", scopeErr)
			return
		}

		if (existing.UserID.Valid && !scopeUser.Valid) || (!existing.UserID.Valid && scopeUser.Valid) || (existing.UserID.Valid && scopeUser.Valid && existing.UserID.UUID != scopeUser.UUID) {
			scopeErr := errors.New("scope update not supported")
			if respondWithAINotice(w, r, "error", "Scope cannot be changed on update", scopeErr) {
				return
			}
			RespondWithError(w, http.StatusBadRequest, "scope cannot be changed on update", scopeErr)
			return
		}

		scopeUser = existing.UserID
		hasExisting = true
		status = http.StatusOK

		if ok, msg := canManageCredentialScope(session, scopeUser); !ok {
			permErr := errors.New("insufficient credential permissions")
			if respondWithAINotice(w, r, "error", msg, permErr) {
				return
			}
			RespondWithError(w, http.StatusForbidden, "insufficient permissions", permErr)
			return
		}
	}

	if hasExisting {
		if _, ok := metadata[config.MetadataKeyCredentialSuffix]; !ok {
			if suffix, ok := credentialSuffixFromMetadata(existing.Metadata); ok {
				metadata[config.MetadataKeyCredentialSuffix] = suffix
			}
		}
	}

	if req.APIKey != "" {
		suffix := deriveCredentialSuffix(providerID, req.APIKey)
		if suffix != "" {
			metadata[config.MetadataKeyCredentialSuffix] = suffix
		} else {
			delete(metadata, config.MetadataKeyCredentialSuffix)
		}
	}

	var credentialCipher []byte
	var credentialHash []byte
	switch {
	case req.APIKey != "":
		ciphertext, err := h.CredentialCipher.Encrypt(ctx, []byte(req.APIKey))
		if err != nil {
			if respondWithAINotice(w, r, "error", "Failed to encrypt credential", err) {
				return
			}
			RespondWithError(w, http.StatusInternalServerError, "failed to encrypt credential", err)
			return
		}
		credentialCipher = ciphertext
		credentialHash = hashSecret([]byte(req.APIKey))
	case hasExisting:
		credentialCipher = append([]byte(nil), existing.CredentialCipher...)
		credentialHash = append([]byte(nil), existing.CredentialHash...)
	default:
		missingErr := errors.New("missing api key")
		if respondWithAINotice(w, r, "error", "API key is required", missingErr) {
			return
		}
		RespondWithError(w, http.StatusBadRequest, "apiKey is required", missingErr)
		return
	}

	record := ai.CredentialRecord{
		CompanyID:        session.CompanyID,
		UserID:           scopeUser,
		ProviderID:       providerID,
		CredentialCipher: credentialCipher,
		CredentialHash:   credentialHash,
		Metadata:         metadata,
		Label:            labelPtr,
		IsDefault:        req.MakeDefault,
	}

	if hasExisting {
		record.ID = existing.ID
		record.UserID = existing.UserID
	}

	if record.IsDefault {
		if err := h.CredentialStore.ClearDefault(ctx, session.CompanyID, providerID, record.UserID); err != nil {
			if respondWithAINotice(w, r, "error", "Failed to update default credential", err) {
				return
			}
			RespondWithError(w, http.StatusInternalServerError, "failed to update default credential", err)
			return
		}
	}

	stored, err := h.CredentialStore.UpsertCredential(ctx, record)
	if err != nil {
		if respondWithAINotice(w, r, "error", "Failed to store credential", err) {
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to store credential", err)
		return
	}

	action := "update"
	if status == http.StatusCreated {
		action = "create"
	}
	eventMeta := map[string]any{
		"credential_id":     stored.ID.String(),
		"fingerprint":       stored.Fingerprint,
		"scope":             scopeFromUserID(stored.UserID),
		"is_default":        stored.IsDefault,
		"requested_default": req.MakeDefault,
	}
	if stored.Label != nil && *stored.Label != "" {
		eventMeta["label"] = *stored.Label
	}
	if stored.UserID.Valid {
		eventMeta["user_id"] = stored.UserID.UUID.String()
	}
	if keys := mapKeys(metadata); len(keys) > 0 {
		eventMeta["metadata_keys"] = keys
	}
	h.recordCredentialEvent(ctx, session.CompanyID, stored.UserID, session.UserID, providerID, action, eventMeta)

	triggerCredentialRefresh(w)
	successMessage := "Credential updated"
	if status == http.StatusCreated {
		successMessage = "Credential added"
	}
	if respondWithAINotice(w, r, "success", successMessage, nil) {
		return
	}

	RespondWithJSON(w, status, credentialRecordToResponse(stored))
}
