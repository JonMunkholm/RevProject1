package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/ai/conversation"
	"github.com/JonMunkholm/RevProject1/internal/ai/documents"
	catalog "github.com/JonMunkholm/RevProject1/internal/ai/provider/catalog"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

const (
	defaultConversationLimit    int32 = 20
	defaultJobLimit             int32 = 20
	defaultCredentialLimit      int32 = 20
	defaultCredentialEventLimit int32 = 20

	metadataKeyCredentialSuffix = "key_suffix"
)

var errStatusNotImplemented = errors.New("ai: provider status check not implemented")

type credentialEventStore interface {
	Insert(ctx context.Context, params database.InsertAIProviderCredentialEventParams) error
	List(ctx context.Context, companyID uuid.UUID, providerID string, action *string, scope *string, actor uuid.NullUUID, limit, offset int32) ([]database.AiProviderCredentialEvent, error)
}

// AI exposes HTTP handlers for conversation and document job workflows.
type AI struct {
	Conversations     *ai.ConversationService
	Documents         *ai.DocumentService
	DefaultProvider   string
	Client            *ai.Client
	Resolver          ai.CredentialResolver
	APIKey            string
	CredentialStore   ai.CredentialStore
	CredentialCipher  ai.CredentialCipher
	CredentialEvents  credentialEventStore
	CredentialMetrics ai.CredentialMetrics
	ProviderCatalog   []ai.ProviderCatalogEntry
	CatalogLoader     *catalog.Loader
}

type conversationResponse struct {
	ID        string         `json:"id"`
	Title     string         `json:"title,omitempty"`
	Provider  string         `json:"provider"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type messageResponse struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

type documentJobResponse struct {
	ID          string         `json:"id"`
	Provider    string         `json:"provider"`
	Status      string         `json:"status"`
	Request     map[string]any `json:"request"`
	Response    map[string]any `json:"response,omitempty"`
	Error       *string        `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
}

type createConversationRequest struct {
	Title    string         `json:"title,omitempty"`
	Provider string         `json:"provider,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type appendMessageRequest struct {
	Content  string         `json:"content"`
	Role     string         `json:"role,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type createDocumentJobRequest struct {
	Provider     string         `json:"provider,omitempty"`
	Documents    []string       `json:"documents"`
	Instructions string         `json:"instructions,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type upsertProviderCredentialRequest struct {
	Provider     string         `json:"provider,omitempty"`
	APIKey       string         `json:"apiKey"`
	Model        string         `json:"model,omitempty"`
	BaseURL      string         `json:"baseUrl,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Scope        string         `json:"scope,omitempty"`
	UserID       string         `json:"userId,omitempty"`
	Label        string         `json:"label,omitempty"`
	MakeDefault  bool           `json:"makeDefault,omitempty"`
	CredentialID string         `json:"credentialId,omitempty"`
}

type listResponse[T any] struct {
	Items      []T   `json:"items"`
	NextOffset int32 `json:"nextOffset"`
}

type providerCredentialResponse struct {
	ID          string         `json:"id"`
	ProviderID  string         `json:"provider"`
	Scope       string         `json:"scope"`
	UserID      *string        `json:"userId,omitempty"`
	Label       string         `json:"label,omitempty"`
	Fingerprint string         `json:"fingerprint"`
	IsDefault   bool           `json:"isDefault"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	LastUsedAt  *time.Time     `json:"lastUsedAt,omitempty"`
	RotatedAt   *time.Time     `json:"rotatedAt,omitempty"`
	KeySuffix   string         `json:"keySuffix,omitempty"`
}

type providerCredentialEventResponse struct {
	ID        string         `json:"id"`
	Action    string         `json:"action"`
	ActorID   *string        `json:"actorId,omitempty"`
	UserID    *string        `json:"userId,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

func credentialRecordToResponse(record ai.CredentialRecord) providerCredentialResponse {
	var userID *string
	if record.UserID.Valid {
		id := record.UserID.UUID.String()
		userID = &id
	}

	label := ""
	if record.Label != nil {
		label = *record.Label
	}

	scope := "company"
	if record.UserID.Valid {
		scope = "user"
	}

	meta := record.Metadata
	if meta == nil {
		meta = map[string]any{}
	}

	var keySuffix string
	if suffix, ok := credentialSuffixFromMetadata(meta); ok {
		keySuffix = suffix
	}

	return providerCredentialResponse{
		ID:          record.ID.String(),
		ProviderID:  record.ProviderID,
		Scope:       scope,
		UserID:      userID,
		Label:       label,
		Fingerprint: record.Fingerprint,
		IsDefault:   record.IsDefault,
		Metadata:    meta,
		UpdatedAt:   record.UpdatedAt,
		LastUsedAt:  record.LastUsedAt,
		RotatedAt:   record.RotatedAt,
		KeySuffix:   keySuffix,
	}
}

func credentialRecordToPageView(record ai.CredentialRecord) pages.AICredentialView {
	resp := credentialRecordToResponse(record)
	view := pages.AICredentialView{
		ID:          resp.ID,
		Provider:    resp.ProviderID,
		Scope:       resp.Scope,
		UserID:      resp.UserID,
		Label:       resp.Label,
		Fingerprint: resp.Fingerprint,
		KeySuffix:   resp.KeySuffix,
		Metadata:    resp.Metadata,
		UpdatedAt:   resp.UpdatedAt,
		LastUsedAt:  resp.LastUsedAt,
		RotatedAt:   resp.RotatedAt,
		IsDefault:   resp.IsDefault,
	}

	scopeLabel := "Company"
	if resp.Scope == "user" {
		scopeLabel = "User"
	}
	if resp.IsDefault {
		scopeLabel += " · Default"
	}
	view.ScopeLabel = scopeLabel

	return view
}

func credentialEventToResponse(event database.AiProviderCredentialEvent, includeSensitive bool) providerCredentialEventResponse {
	var actorID *string
	if includeSensitive && event.ActorUserID.Valid {
		id := event.ActorUserID.UUID.String()
		actorID = &id
	}

	var userID *string
	if event.UserID.Valid {
		id := event.UserID.UUID.String()
		userID = &id
	}

	meta := decodeEventMetadata(event.MetadataSnapshot)
	if !includeSensitive {
		delete(meta, "fingerprint")
	}

	return providerCredentialEventResponse{
		ID:        event.ID.String(),
		Action:    event.Action,
		ActorID:   actorID,
		UserID:    userID,
		Metadata:  meta,
		CreatedAt: event.CreatedAt,
	}
}

func decodeEventMetadata(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"error": err.Error()}
	}
	return out
}

func credentialEventToPageView(event database.AiProviderCredentialEvent, includeSensitive bool) pages.AICredentialEventView {
	resp := credentialEventToResponse(event, includeSensitive)
	return pages.AICredentialEventView{
		ID:        resp.ID,
		Action:    resp.Action,
		ActorID:   resp.ActorID,
		UserID:    resp.UserID,
		Metadata:  resp.Metadata,
		CreatedAt: resp.CreatedAt,
	}
}

// ListProviderCatalog returns metadata about supported AI providers.
func (h *AI) ListProviderCatalog(w http.ResponseWriter, r *http.Request) {
	entries := h.catalogEntries(r.Context())
	RespondWithJSON(w, http.StatusOK, struct {
		Items []ai.ProviderCatalogEntry `json:"items"`
	}{Items: entries})
}

// ListProviderCredentials returns credential metadata for the current company.
func (h *AI) ListProviderCredentials(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.CredentialStore == nil {
		RespondWithError(w, http.StatusInternalServerError, "credentials unavailable", errors.New("credential store not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	if !session.Capabilities.CanViewProviderCredentials {
		RespondWithError(w, http.StatusForbidden, "insufficient permissions", errors.New("view not permitted"))
		return
	}

	limit, offset := paginationParams(r, defaultCredentialLimit)
	providerFilter := strings.TrimSpace(r.URL.Query().Get("provider"))
	if providerPath := strings.TrimSpace(chi.URLParam(r, "providerID")); providerPath != "" {
		providerFilter = providerPath
	}
	scopeFilter := strings.TrimSpace(r.URL.Query().Get("scope"))
	userFilter := strings.TrimSpace(r.URL.Query().Get("userId"))

	if providerFilter != "" {
		if _, _, err := h.normalizeProvider(r.Context(), providerFilter); err != nil {
			RespondWithError(w, http.StatusBadRequest, "unknown provider", err)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var (
		records    []ai.CredentialRecord
		err        error
		nextOffset int32 = offset + limit
	)

	if providerFilter != "" {
		nextOffset = 0
		if scopeFilter != "" || userFilter != "" {
			var userID uuid.NullUUID
			userID, _, err = resolveCredentialScope(session, scopeFilter, userFilter)
			if err != nil {
				RespondWithError(w, http.StatusBadRequest, "invalid scope", err)
				return
			}
			records, err = h.CredentialStore.ListProviderCredentials(ctx, session.CompanyID, providerFilter, userID)
		} else {
			records, err = h.collectProviderCredentialsForSession(ctx, session, providerFilter)
		}
	} else {
		records, err = h.CredentialStore.ListCompanyCredentials(ctx, session.CompanyID, limit, offset)
	}
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to load credentials", err)
		return
	}

	if isHTMX(r) {
		views := make([]pages.AICredentialView, 0, len(records))
		for _, record := range records {
			views = append(views, credentialRecordToPageView(record))
		}
		if err := renderCredentialTable(r.Context(), w, views); err != nil {
			RespondWithError(w, http.StatusInternalServerError, "failed to render credentials", err)
		}
		return
	}

	resp := listResponse[providerCredentialResponse]{NextOffset: nextOffset}
	for _, record := range records {
		resp.Items = append(resp.Items, credentialRecordToResponse(record))
	}

	RespondWithJSON(w, http.StatusOK, resp)
}

// ListProviderCredentialEvents returns audit logs for a provider scoped to the current company.
func (h *AI) ListProviderCredentialEvents(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.CredentialEvents == nil {
		RespondWithError(w, http.StatusInternalServerError, "credential events unavailable", errors.New("credential events store not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	if !session.Capabilities.CanViewProviderCredentials {
		RespondWithError(w, http.StatusForbidden, "insufficient permissions", errors.New("view not permitted"))
		return
	}

	providerParam := strings.TrimSpace(chi.URLParam(r, "providerID"))
	providerID, _, err := h.normalizeProvider(r.Context(), providerParam)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "unknown provider", err)
		return
	}

	limit, offset := paginationParams(r, defaultCredentialEventLimit)
	actionFilter := strings.TrimSpace(r.URL.Query().Get("action"))
	scopeFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scopeFilter != "" && scopeFilter != "company" && scopeFilter != "user" {
		RespondWithError(w, http.StatusBadRequest, "invalid scope filter", fmt.Errorf("unsupported scope %q", scopeFilter))
		return
	}
	actorFilter := strings.TrimSpace(r.URL.Query().Get("actorId"))

	var actionPtr *string
	if actionFilter != "" {
		actionPtr = &actionFilter
	}
	var scopePtr *string
	if scopeFilter != "" {
		scopePtr = &scopeFilter
	}
	actorUUID := uuid.NullUUID{}
	if actorFilter != "" {
		parsed, err := uuid.Parse(actorFilter)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "invalid actorId", err)
			return
		}
		actorUUID = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	events, err := h.CredentialEvents.List(ctx, session.CompanyID, providerID, actionPtr, scopePtr, actorUUID, limit, offset)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to load credential events", err)
		return
	}

	includeSensitive := session.Capabilities.CanManageCompanyCredentials
	if isHTMX(r) {
		views := make([]pages.AICredentialEventView, 0, len(events))
		for _, event := range events {
			views = append(views, credentialEventToPageView(event, includeSensitive))
		}
		if err := renderCredentialEvents(r.Context(), w, views); err != nil {
			RespondWithError(w, http.StatusInternalServerError, "failed to render credential events", err)
		}
		return
	}

	resp := listResponse[providerCredentialEventResponse]{NextOffset: offset + limit}
	for _, event := range events {
		resp.Items = append(resp.Items, credentialEventToResponse(event, includeSensitive))
	}

	RespondWithJSON(w, http.StatusOK, resp)
}

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

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
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

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
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

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
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

	// TODO: implement provider ping once provider clients support it.
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

// CreateConversation starts a new conversation session for the authenticated company/user.
func (h *AI) CreateConversation(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Conversations == nil {
		RespondWithError(w, http.StatusInternalServerError, "ai conversations unavailable", errors.New("conversation service not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	var req createConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		return
	}

	providerID := req.Provider
	if providerID == "" {
		providerID = h.DefaultProvider
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	record, err := h.Conversations.StartSession(ctx, conversation.CreateSessionParams{
		CompanyID:  session.CompanyID,
		UserID:     session.UserID,
		ProviderID: providerID,
		Title:      req.Title,
		Metadata:   req.Metadata,
	})
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to create conversation", err)
		return
	}

	RespondWithJSON(w, http.StatusCreated, sessionToResponse(record))
}

// ListConversations returns paginated conversation sessions for the authenticated company.
func (h *AI) ListConversations(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Conversations == nil {
		RespondWithError(w, http.StatusInternalServerError, "ai conversations unavailable", errors.New("conversation service not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	limit, offset := paginationParams(r, defaultConversationLimit)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	records, err := h.Conversations.ListCompanySessions(ctx, session.CompanyID, limit, offset)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to list conversations", err)
		return
	}

	resp := listResponse[conversationResponse]{NextOffset: offset + limit}
	for _, record := range records {
		resp.Items = append(resp.Items, sessionToResponse(record))
	}

	RespondWithJSON(w, http.StatusOK, resp)
}

// ListConversationMessages returns messages for a given session.
func (h *AI) ListConversationMessages(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Conversations == nil {
		RespondWithError(w, http.StatusInternalServerError, "ai conversations unavailable", errors.New("conversation service not configured"))
		return
	}

	sessionInfo, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid session id", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if _, err := h.Conversations.Session(ctx, sessionInfo.CompanyID, sessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			RespondWithError(w, http.StatusNotFound, "conversation not found", err)
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to load conversation", err)
		return
	}

	messages, err := h.Conversations.ListSessionMessages(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			RespondWithError(w, http.StatusNotFound, "conversation not found", err)
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to list messages", err)
		return
	}

	resp := listResponse[messageResponse]{NextOffset: 0}
	for _, msg := range messages {
		resp.Items = append(resp.Items, messageToResponse(msg))
	}

	RespondWithJSON(w, http.StatusOK, resp)
}

// AppendConversationMessage stores a user message and synchronously generates an assistant reply.
func (h *AI) AppendConversationMessage(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Conversations == nil {
		RespondWithError(w, http.StatusInternalServerError, "ai conversations unavailable", errors.New("conversation service not configured"))
		return
	}

	sessionInfo, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid session id", err)
		return
	}

	var req appendMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		return
	}
	if req.Content == "" {
		RespondWithError(w, http.StatusBadRequest, "content is required", errors.New("missing content"))
		return
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	sessionRecord, err := h.Conversations.Session(ctx, sessionInfo.CompanyID, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			RespondWithError(w, http.StatusNotFound, "conversation not found", err)
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to load conversation", err)
		return
	}

	msg, err := h.Conversations.AppendMessage(ctx, conversation.CreateMessageParams{
		SessionID: sessionID,
		Role:      role,
		Content:   req.Content,
		Metadata:  req.Metadata,
	})
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to save message", err)
		return
	}

	if h.Client == nil {
		RespondWithJSON(w, http.StatusAccepted, messageToResponse(msg))
		return
	}

	messages, err := h.Conversations.ListSessionMessages(ctx, sessionID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to load message history", err)
		return
	}

	metadata := mergeMetadataMaps(sessionRecord.Metadata, req.Metadata)
	completionMetadata := map[string]any{}
	if addendum, ok := metadata["system_addendum"].(string); ok && addendum != "" {
		completionMetadata = ai.WithSystemAddendum(completionMetadata, addendum)
	}

	options := h.userOptions(ctx, sessionInfo.CompanyID, sessionInfo.UserID, sessionRecord.ProviderID)
	prompt := buildConversationPrompt(messages)
	resp, err := h.Client.Completion(ctx, options, ai.CompletionRequest{Prompt: prompt, Metadata: completionMetadata})
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to generate reply", err)
		return
	}

	reply, err := h.Conversations.AppendMessage(ctx, conversation.CreateMessageParams{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   strings.TrimSpace(resp.Text),
		Metadata: map[string]any{
			"provider": sessionRecord.ProviderID,
		},
	})
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to persist reply", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, struct {
		Message messageResponse `json:"message"`
	}{Message: messageToResponse(reply)})
}

// CreateDocumentJob queues a document analysis request.
func (h *AI) CreateDocumentJob(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Documents == nil {
		RespondWithError(w, http.StatusInternalServerError, "document analysis unavailable", errors.New("document service not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	var req createDocumentJobRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		return
	}
	if len(req.Documents) == 0 {
		RespondWithError(w, http.StatusBadRequest, "documents are required", errors.New("missing documents"))
		return
	}

	providerID := req.Provider
	if providerID == "" {
		providerID = h.DefaultProvider
	}

	requestPayload := map[string]any{
		"documents": req.Documents,
	}
	if req.Instructions != "" {
		requestPayload["instructions"] = req.Instructions
	}
	if len(req.Metadata) > 0 {
		requestPayload["metadata"] = req.Metadata
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	job, err := h.Documents.Enqueue(ctx, documents.CreateJobParams{
		CompanyID:  session.CompanyID,
		UserID:     session.UserID,
		ProviderID: providerID,
		Request:    requestPayload,
	})
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to queue job", err)
		return
	}

	RespondWithJSON(w, http.StatusAccepted, jobToResponse(job))
}

// ListDocumentJobs lists queued and historical jobs for the company.
func (h *AI) ListDocumentJobs(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Documents == nil {
		RespondWithError(w, http.StatusInternalServerError, "document analysis unavailable", errors.New("document service not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	limit, offset := paginationParams(r, defaultJobLimit)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	jobs, err := h.Documents.Jobs(ctx, session.CompanyID, limit, offset)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to list jobs", err)
		return
	}

	resp := listResponse[documentJobResponse]{NextOffset: offset + limit}
	for _, job := range jobs {
		resp.Items = append(resp.Items, jobToResponse(job))
	}

	RespondWithJSON(w, http.StatusOK, resp)
}

// GetDocumentJob returns a single job by ID.
func (h *AI) GetDocumentJob(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Documents == nil {
		RespondWithError(w, http.StatusInternalServerError, "document analysis unavailable", errors.New("document service not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid job id", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	job, err := h.Documents.Job(ctx, session.CompanyID, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			RespondWithError(w, http.StatusNotFound, "job not found", err)
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to load job", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, jobToResponse(job))
}

func sessionToResponse(record conversation.Session) conversationResponse {
	return conversationResponse{
		ID:        record.ID.String(),
		Title:     record.Title,
		Provider:  record.ProviderID,
		Metadata:  record.Metadata,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}

func messageToResponse(record conversation.Message) messageResponse {
	return messageResponse{
		ID:        record.ID.String(),
		Role:      record.Role,
		Content:   record.Content,
		Metadata:  record.Metadata,
		CreatedAt: record.CreatedAt,
	}
}

func jobToResponse(job documents.Job) documentJobResponse {
	return documentJobResponse{
		ID:          job.ID.String(),
		Provider:    job.ProviderID,
		Status:      job.Status,
		Request:     job.Request,
		Response:    job.Response,
		Error:       job.ErrorMessage,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		CompletedAt: job.CompletedAt,
	}
}

func paginationParams(r *http.Request, fallback int32) (int32, int32) {
	limit := fallback
	offset := int32(0)

	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = int32(v)
		}
	}
	return limit, offset
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

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
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
		if _, ok := metadata[metadataKeyCredentialSuffix]; !ok {
			if suffix, ok := credentialSuffixFromMetadata(existing.Metadata); ok {
				metadata[metadataKeyCredentialSuffix] = suffix
			}
		}
	}

	if req.APIKey != "" {
		suffix := deriveCredentialSuffix(providerID, req.APIKey)
		if suffix != "" {
			metadata[metadataKeyCredentialSuffix] = suffix
		} else {
			delete(metadata, metadataKeyCredentialSuffix)
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

func hashSecret(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
}

func (h *AI) userOptions(ctx context.Context, companyID, userID uuid.UUID, providerID string) ai.UserOptions {
	if providerID == "" {
		providerID = h.DefaultProvider
	}
	opts := ai.UserOptions{Provider: providerID}
	if h.APIKey != "" {
		opts.APIKey = h.APIKey
	}
	if opts.APIKey == "" && h.Resolver != nil {
		reference := ai.CredentialReference{CompanyID: companyID, UserID: userID, ProviderID: providerID}
		if key, err := h.Resolver.Resolve(ctx, reference.String()); err == nil && key != "" {
			opts.APIKey = key
		} else if err != nil && h.CredentialMetrics != nil {
			h.CredentialMetrics.CredentialResolveFailure(companyID, providerID)
		}
	}
	if opts.APIKey == "" && h.CredentialMetrics != nil {
		scope := "user"
		if userID == uuid.Nil {
			scope = "company"
		}
		h.CredentialMetrics.CredentialMissing(companyID, providerID, scope)
	}
	return opts
}

func mergeMetadataMaps(values ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, item := range values {
		for k, v := range item {
			out[k] = v
		}
	}
	return out
}

func buildConversationPrompt(messages []conversation.Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		role := msg.Role
		if role == "" {
			role = "user"
		}
		builder.WriteString(strings.ToUpper(role))
		builder.WriteString(": ")
		builder.WriteString(msg.Content)
		builder.WriteString("\n\n")
	}
	return builder.String()
}

func resolveCredentialScope(session auth.Session, scope, userIDParam string) (uuid.NullUUID, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(scope))
	if normalized == "" {
		normalized = "user"
	}

	switch normalized {
	case "user":
		target := session.UserID
		if userIDParam != "" {
			parsed, err := uuid.Parse(userIDParam)
			if err != nil {
				return uuid.NullUUID{}, "", fmt.Errorf("invalid userId: %w", err)
			}
			target = parsed
		}
		if target == uuid.Nil {
			return uuid.NullUUID{}, "", errors.New("user scope requires a userId")
		}
		return uuid.NullUUID{UUID: target, Valid: true}, "user", nil
	case "company":
		return uuid.NullUUID{}, "company", nil
	default:
		return uuid.NullUUID{}, "", fmt.Errorf("invalid scope %q", scope)
	}
}

func (h *AI) collectProviderCredentialsForSession(ctx context.Context, session auth.Session, providerID string) ([]ai.CredentialRecord, error) {
	companyRecords, err := h.CredentialStore.ListProviderCredentials(ctx, session.CompanyID, providerID, uuid.NullUUID{})
	if err != nil {
		return nil, err
	}

	records := make([]ai.CredentialRecord, 0, len(companyRecords))
	records = append(records, companyRecords...)

	if session.UserID != uuid.Nil {
		userScope := uuid.NullUUID{UUID: session.UserID, Valid: true}
		userRecords, err := h.CredentialStore.ListProviderCredentials(ctx, session.CompanyID, providerID, userScope)
		if err != nil {
			return nil, err
		}
		records = append(records, userRecords...)
	}

	sort.SliceStable(records, func(i, j int) bool {
		iCompany := !records[i].UserID.Valid
		jCompany := !records[j].UserID.Valid
		if iCompany != jCompany {
			return iCompany
		}
		if records[i].IsDefault != records[j].IsDefault {
			return records[i].IsDefault && !records[j].IsDefault
		}
		if !records[i].UpdatedAt.Equal(records[j].UpdatedAt) {
			return records[i].UpdatedAt.After(records[j].UpdatedAt)
		}
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})

	return records, nil
}

func renderCredentialTable(ctx context.Context, w http.ResponseWriter, views []pages.AICredentialView) error {
	var buf bytes.Buffer
	buf.WriteString(`<div id="credential-table">`)
	component := pages.AICredentialTable(views)
	if err := component.Render(ctx, &buf); err != nil {
		return err
	}
	buf.WriteString(`</div>`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(buf.Bytes())
	return err
}

func renderCredentialEvents(ctx context.Context, w http.ResponseWriter, views []pages.AICredentialEventView) error {
	var buf bytes.Buffer
	buf.WriteString(`<div id="credential-events" class="ai-settings__events">`)
	if err := pages.AICredentialEventsTable(views).Render(ctx, &buf); err != nil {
		return err
	}
	buf.WriteString(`</div>`)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(buf.Bytes())
	return err
}

func triggerCredentialRefresh(w http.ResponseWriter) {
	const trigger = "ai-credentials-refresh"
	if existing := w.Header().Get("HX-Trigger"); existing != "" {
		w.Header().Set("HX-Trigger", existing+","+trigger)
		return
	}
	w.Header().Set("HX-Trigger", trigger)
}

func isHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func validateAPIKeyFormat(providerID, key string) error {
	switch providerID {
	case "openai":
		if !strings.HasPrefix(key, "sk-") {
			return fmt.Errorf("openai api keys must begin with 'sk-'")
		}
	}
	return nil
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	clone := make(map[string]any, len(metadata))
	for k, v := range metadata {
		clone[k] = v
	}
	return clone
}

func credentialSuffixFromMetadata(metadata map[string]any) (string, bool) {
	if metadata == nil {
		return "", false
	}
	value, ok := metadata[metadataKeyCredentialSuffix]
	if !ok {
		return "", false
	}
	switch v := value.(type) {
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return "", false
		}
		return v, true
	case fmt.Stringer:
		text := strings.TrimSpace(v.String())
		if text == "" {
			return "", false
		}
		return text, true
	default:
		return "", false
	}
}

func deriveCredentialSuffix(_ string, apiKey string) string {
	trimmed := strings.TrimSpace(apiKey)
	if len(trimmed) < 4 {
		return ""
	}
	return trimmed[len(trimmed)-4:]
}

func (h *AI) catalogEntries(ctx context.Context) []ai.ProviderCatalogEntry {
	if h == nil {
		return ai.ProviderCatalog()
	}
	if h.CatalogLoader != nil {
		if entries := h.CatalogLoader.Entries(ctx); len(entries) > 0 {
			return entries
		}
	}
	if len(h.ProviderCatalog) > 0 {
		return h.ProviderCatalog
	}
	return ai.ProviderCatalog()
}

func (h *AI) catalogEntry(ctx context.Context, providerID string) (ai.ProviderCatalogEntry, bool) {
	id := strings.TrimSpace(providerID)
	if id == "" {
		return ai.ProviderCatalogEntry{}, false
	}
	for _, entry := range h.catalogEntries(ctx) {
		if strings.EqualFold(entry.ID, id) {
			return entry, true
		}
	}
	return ai.ProviderCatalogEntry{}, false
}

func (h *AI) normalizeProvider(ctx context.Context, providerID string) (string, ai.ProviderCatalogEntry, error) {
	id := strings.TrimSpace(providerID)
	if id == "" {
		id = h.DefaultProvider
	}
	entry, ok := h.catalogEntry(ctx, id)
	if !ok {
		return "", ai.ProviderCatalogEntry{}, fmt.Errorf("unknown provider %q", id)
	}
	return entry.ID, entry, nil
}

func writeAINotice(ctx context.Context, w http.ResponseWriter, notice pages.SettingsNotice) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := pages.SettingsAINoticePartial(notice).Render(ctx, w); err != nil {
		log.Printf("ai: failed to render notice: %v", err)
	}
}

func writeAIStatusBadge(ctx context.Context, w http.ResponseWriter, badge pages.SettingsStatusBadge) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := pages.SettingsAIStatusBadgePartial(badge).Render(ctx, w); err != nil {
		log.Printf("ai: failed to render status badge: %v", err)
	}
}

func handleProviderStatusError(w http.ResponseWriter, r *http.Request, status int, message string, err error) {
	if isHTMX(r) {
		writeAIStatusBadge(r.Context(), w, pages.SettingsStatusBadge{Status: "error", Message: message})
		if err != nil {
			log.Printf("ai: provider status error: %v", err)
		}
		return
	}
	RespondWithError(w, status, message, err)
}

func respondWithAINotice(w http.ResponseWriter, r *http.Request, status, message string, err error) bool {
	if isHTMX(r) {
		if err != nil {
			log.Printf("ai: credential notice (%s): %v", status, err)
		}
		writeAINotice(r.Context(), w, pages.SettingsNotice{Status: status, Message: message})
		return true
	}
	return false
}

func parseUpsertProviderCredentialRequest(r *http.Request) (upsertProviderCredentialRequest, error) {
	var req upsertProviderCredentialRequest
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	if contentType == "application/json" {
		if err := decodeJSON(r, &req); err != nil {
			return upsertProviderCredentialRequest{}, err
		}
		req.Provider = strings.TrimSpace(req.Provider)
		req.APIKey = strings.TrimSpace(req.APIKey)
		req.Model = strings.TrimSpace(req.Model)
		req.BaseURL = strings.TrimSpace(req.BaseURL)
		req.Scope = strings.TrimSpace(req.Scope)
		req.UserID = strings.TrimSpace(req.UserID)
		req.Label = strings.TrimSpace(req.Label)
		req.CredentialID = strings.TrimSpace(req.CredentialID)
		if req.Metadata == nil {
			req.Metadata = make(map[string]any)
		}
		return req, nil
	}

	if err := r.ParseForm(); err != nil {
		return upsertProviderCredentialRequest{}, err
	}

	form := r.PostForm
	req.Provider = strings.TrimSpace(formValue(form, "provider"))
	req.APIKey = strings.TrimSpace(formValue(form, "apiKey"))
	req.Model = strings.TrimSpace(formValue(form, "model"))
	req.BaseURL = strings.TrimSpace(firstNonEmpty(form, "baseUrl", "base_url"))
	req.Scope = strings.TrimSpace(formValue(form, "scope"))
	req.UserID = strings.TrimSpace(formValue(form, "userId"))
	req.Label = strings.TrimSpace(formValue(form, "label"))
	req.CredentialID = strings.TrimSpace(formValue(form, "credentialId"))
	req.MakeDefault = formValue(form, "makeDefault") != ""
	req.Metadata = make(map[string]any)

	skip := map[string]struct{}{
		"provider":     {},
		"apiKey":       {},
		"model":        {},
		"baseUrl":      {},
		"base_url":     {},
		"scope":        {},
		"userId":       {},
		"label":        {},
		"makeDefault":  {},
		"credentialId": {},
	}

	for key, values := range form {
		if _, ok := skip[key]; ok {
			continue
		}
		if len(values) == 0 {
			continue
		}
		req.Metadata[key] = values[len(values)-1]
	}

	return req, nil
}

func formValue(values url.Values, key string) string {
	return strings.TrimSpace(values.Get(key))
}

func firstNonEmpty(values url.Values, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(values.Get(key)); v != "" {
			return v
		}
	}
	return ""
}

func canManageCredentialScope(session auth.Session, scope uuid.NullUUID) (bool, string) {
	if scope.Valid {
		if scope.UUID == session.UserID {
			if !session.Capabilities.CanManagePersonalCredentials {
				return false, "You do not have permission to manage personal credentials."
			}
		} else {
			if !session.Capabilities.CanManageCompanyCredentials {
				return false, "Admin permissions are required to manage another user's credentials."
			}
		}
	} else {
		if !session.Capabilities.CanManageCompanyCredentials {
			return false, "Company-wide credential requires an admin."
		}
	}
	return true, ""
}

func (h *AI) selectCredentialForStatus(ctx context.Context, companyID uuid.UUID, providerID string, preferredUser uuid.UUID) (ai.CredentialRecord, uuid.NullUUID, string, bool, error) {
	if h == nil || h.CredentialStore == nil {
		return ai.CredentialRecord{}, uuid.NullUUID{}, "", false, nil
	}

	selectRecord := func(records []ai.CredentialRecord) (ai.CredentialRecord, bool) {
		var chosen ai.CredentialRecord
		for _, record := range records {
			if record.IsDefault {
				return record, true
			}
			if chosen.ID == uuid.Nil {
				chosen = record
			}
		}
		if chosen.ID != uuid.Nil {
			return chosen, true
		}
		return ai.CredentialRecord{}, false
	}

	companyRecords, err := h.CredentialStore.ListProviderCredentials(ctx, companyID, providerID, uuid.NullUUID{})
	if err != nil {
		return ai.CredentialRecord{}, uuid.NullUUID{}, "", false, err
	}
	if record, ok := selectRecord(companyRecords); ok {
		return record, uuid.NullUUID{}, "company", true, nil
	}

	if preferredUser != uuid.Nil {
		userID := uuid.NullUUID{UUID: preferredUser, Valid: true}
		userRecords, err := h.CredentialStore.ListProviderCredentials(ctx, companyID, providerID, userID)
		if err != nil {
			return ai.CredentialRecord{}, uuid.NullUUID{}, "", false, err
		}
		if record, ok := selectRecord(userRecords); ok {
			return record, userID, "user", true, nil
		}
	}

	return ai.CredentialRecord{}, uuid.NullUUID{}, "", false, nil
}

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	if value, ok := meta[key]; ok {
		if str, ok := value.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}

func (h *AI) pingProvider(ctx context.Context, providerID string, entry ai.ProviderCatalogEntry, apiKey string, metadata map[string]any) error {
	switch providerID {
	case "openai":
		baseURL := metadataString(metadata, "base_url")
		if baseURL == "" {
			baseURL = metadataString(metadata, "baseUrl")
		}
		return pingOpenAI(ctx, baseURL, apiKey)
	default:
		return fmt.Errorf("%w: %s", errStatusNotImplemented, providerID)
	}
}

func pingOpenAI(ctx context.Context, baseURL, apiKey string) error {
	if apiKey == "" {
		return errors.New("missing api key")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	url := baseURL + "/models?limit=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("openai status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (h *AI) recordCredentialEvent(ctx context.Context, companyID uuid.UUID, scopeUser uuid.NullUUID, actor uuid.UUID, providerID, action string, metadata map[string]any) {
	if h == nil || h.CredentialEvents == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	entry := database.InsertAIProviderCredentialEventParams{
		CompanyID:        companyID,
		UserID:           scopeUser,
		ActorUserID:      nullUUID(actor),
		ProviderID:       providerID,
		Action:           action,
		MetadataSnapshot: metadata,
	}
	if err := h.CredentialEvents.Insert(ctx, entry); err != nil {
		log.Printf("ai: failed to record credential event action=%s provider=%s company=%s: %v", action, providerID, companyID, err)
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func scopeFromUserID(user uuid.NullUUID) string {
	if user.Valid {
		return "user"
	}
	return "company"
}

func nullUUID(id uuid.UUID) uuid.NullUUID {
	if id == uuid.Nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: id, Valid: true}
}
