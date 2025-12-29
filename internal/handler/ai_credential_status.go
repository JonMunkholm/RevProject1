package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/config"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

// ProviderStatus performs a lightweight provider ping using stored credentials or the global API key.
func (h *AI) ProviderStatus(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Client == nil {
		handleProviderStatusError(w, r, http.StatusInternalServerError, "AI client unavailable", errors.New("ai client not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		handleProviderStatusError(w, r, http.StatusUnauthorized, "Authentication required", errors.New("session missing"))
		return
	}

	providerParam := chi.URLParam(r, "providerID")
	providerID, entry, err := h.normalizeProvider(r.Context(), providerParam)
	if err != nil {
		handleProviderStatusError(w, r, http.StatusBadRequest, "Unknown provider", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), config.DefaultHandlerTimeout)
	defer cancel()

	scopeLabel := "company"
	scopeUser := uuid.NullUUID{}
	var credentialRecord ai.CredentialRecord
	var metadata map[string]any
	var apiKey string
	var fromCredential bool
	htmx := isHTMX(r)

	if strings.TrimSpace(h.APIKey) != "" {
		apiKey = strings.TrimSpace(h.APIKey)
		scopeLabel = "global"
	} else {
		record, userID, scope, found, err := h.selectCredentialForStatus(ctx, session.CompanyID, providerID, session.UserID)
		if err != nil {
			handleProviderStatusError(w, r, http.StatusInternalServerError, "Failed to load credentials", err)
			return
		}
		if !found {
			if h.CredentialMetrics != nil {
				h.CredentialMetrics.CredentialMissing(session.CompanyID, providerID, scopeLabel)
			}
			meta := map[string]any{"status": "missing", "scope": scopeLabel}
			h.recordCredentialEvent(ctx, session.CompanyID, scopeUser, session.UserID, providerID, "status", meta)
			if htmx {
				writeAIStatusBadge(ctx, w, pages.SettingsStatusBadge{Status: "warning", Message: "No credential configured"})
			} else {
				RespondWithError(w, http.StatusFailedDependency, "no credential configured", errors.New("credential missing"))
			}
			return
		}

		if h.CredentialCipher == nil {
			handleProviderStatusError(w, r, http.StatusInternalServerError, "Credential cipher unavailable", errors.New("credential cipher not configured"))
			return
		}

		plaintext, err := h.CredentialCipher.Decrypt(ctx, record.CredentialCipher)
		if err != nil {
			handleProviderStatusError(w, r, http.StatusInternalServerError, "Failed to decrypt credential", err)
			return
		}

		credentialRecord = record
		metadata = record.Metadata
		apiKey = strings.TrimSpace(string(plaintext))
		scopeUser = userID
		scopeLabel = scope
		fromCredential = true
	}

	if apiKey == "" {
		if h.CredentialMetrics != nil {
			h.CredentialMetrics.CredentialMissing(session.CompanyID, providerID, scopeLabel)
		}
		meta := map[string]any{"status": "missing", "scope": scopeLabel, "reason": "api key empty"}
		h.recordCredentialEvent(ctx, session.CompanyID, scopeUser, session.UserID, providerID, "status", meta)
		if htmx {
			writeAIStatusBadge(ctx, w, pages.SettingsStatusBadge{Status: "warning", Message: "No credential configured"})
		} else {
			RespondWithError(w, http.StatusFailedDependency, "no credential configured", errors.New("credential missing"))
		}
		return
	}

	start := time.Now()
	if err := h.pingProvider(ctx, providerID, entry, apiKey, metadata); err != nil {
		if errors.Is(err, errStatusNotImplemented) {
			meta := map[string]any{"status": "skipped", "scope": scopeLabel, "reason": err.Error()}
			h.recordCredentialEvent(ctx, session.CompanyID, scopeUser, session.UserID, providerID, "status", meta)
			if htmx {
				writeAIStatusBadge(ctx, w, pages.SettingsStatusBadge{Status: "warning", Message: "Status check not implemented"})
			} else {
				RespondWithError(w, http.StatusNotImplemented, "status check not implemented", err)
			}
			return
		}
		if h.CredentialMetrics != nil {
			h.CredentialMetrics.CredentialTestFailure(session.CompanyID, providerID)
		}
		meta := map[string]any{"status": "failure", "scope": scopeLabel, "error": err.Error()}
		if fromCredential {
			meta["credential_id"] = credentialRecord.ID.String()
			if credentialRecord.Fingerprint != "" {
				meta["fingerprint"] = credentialRecord.Fingerprint
			}
			if credentialRecord.UserID.Valid {
				meta["user_id"] = credentialRecord.UserID.UUID.String()
			}
		}
		h.recordCredentialEvent(ctx, session.CompanyID, scopeUser, session.UserID, providerID, "status", meta)
		if htmx {
			writeAIStatusBadge(ctx, w, pages.SettingsStatusBadge{Status: "error", Message: "Status check failed"})
		} else {
			RespondWithError(w, http.StatusBadGateway, "provider status check failed", err)
		}
		return
	}

	latency := time.Since(start)
	eventMeta := map[string]any{"status": "success", "scope": scopeLabel, "latency_ms": latency.Milliseconds()}
	if fromCredential {
		eventMeta["credential_id"] = credentialRecord.ID.String()
		if credentialRecord.Fingerprint != "" {
			eventMeta["fingerprint"] = credentialRecord.Fingerprint
		}
		if credentialRecord.UserID.Valid {
			eventMeta["user_id"] = credentialRecord.UserID.UUID.String()
		}
	}
	h.recordCredentialEvent(ctx, session.CompanyID, scopeUser, session.UserID, providerID, "status", eventMeta)

	if htmx {
		writeAIStatusBadge(ctx, w, pages.SettingsStatusBadge{Status: "ok", Message: "Connected"})
		return
	}

	RespondWithJSON(w, http.StatusOK, map[string]any{
		"provider":  providerID,
		"status":    "ok",
		"scope":     scopeLabel,
		"latencyMs": latency.Milliseconds(),
		"checkedAt": time.Now().UTC(),
	})
}
