package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/ai/conversation"
	"github.com/JonMunkholm/RevProject1/internal/ai/documents"
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
)

type credentialEventStore interface {
	Insert(ctx context.Context, params database.InsertAIProviderCredentialEventParams) error
	List(ctx context.Context, companyID uuid.UUID, providerID string, limit, offset int32) ([]database.AiProviderCredentialEvent, error)
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

func credentialEventToResponse(event database.AiProviderCredentialEvent) providerCredentialEventResponse {
	var actorID *string
	if event.ActorUserID.Valid {
		id := event.ActorUserID.UUID.String()
		actorID = &id
	}

	var userID *string
	if event.UserID.Valid {
		id := event.UserID.UUID.String()
		userID = &id
	}

	return providerCredentialEventResponse{
		ID:        event.ID.String(),
		Action:    event.Action,
		ActorID:   actorID,
		UserID:    userID,
		Metadata:  decodeEventMetadata(event.MetadataSnapshot),
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

func credentialEventToPageView(event database.AiProviderCredentialEvent) pages.AICredentialEventView {
	resp := credentialEventToResponse(event)
	return pages.AICredentialEventView{
		ID:        resp.ID,
		Action:    resp.Action,
		ActorID:   resp.ActorID,
		UserID:    resp.UserID,
		Metadata:  resp.Metadata,
		CreatedAt: resp.CreatedAt,
	}
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

	limit, offset := paginationParams(r, defaultCredentialLimit)
	providerFilter := strings.TrimSpace(r.URL.Query().Get("provider"))
	scopeFilter := strings.TrimSpace(r.URL.Query().Get("scope"))
	userFilter := strings.TrimSpace(r.URL.Query().Get("userId"))

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var (
		records    []ai.CredentialRecord
		err        error
		nextOffset int32 = offset + limit
	)

	if providerFilter != "" {
		var userID uuid.NullUUID
		if scopeFilter != "" || userFilter != "" {
			userID, _, err = resolveCredentialScope(session, scopeFilter, userFilter)
			if err != nil {
				RespondWithError(w, http.StatusBadRequest, "invalid scope", err)
				return
			}
		}
		records, err = h.CredentialStore.ListProviderCredentials(ctx, session.CompanyID, providerFilter, userID)
		nextOffset = 0
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

	providerID := strings.TrimSpace(chi.URLParam(r, "providerID"))

	limit, offset := paginationParams(r, defaultCredentialEventLimit)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	events, err := h.CredentialEvents.List(ctx, session.CompanyID, providerID, limit, offset)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to load credential events", err)
		return
	}

	if isHTMX(r) {
		views := make([]pages.AICredentialEventView, 0, len(events))
		for _, event := range events {
			views = append(views, credentialEventToPageView(event))
		}
		if err := renderCredentialEvents(r.Context(), w, views); err != nil {
			RespondWithError(w, http.StatusInternalServerError, "failed to render credential events", err)
		}
		return
	}

	resp := listResponse[providerCredentialEventResponse]{NextOffset: offset + limit}
	for _, event := range events {
		resp.Items = append(resp.Items, credentialEventToResponse(event))
	}

	RespondWithJSON(w, http.StatusOK, resp)
}

// DeleteProviderCredential removes a credential by identifier.
func (h *AI) DeleteProviderCredential(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.CredentialStore == nil {
		RespondWithError(w, http.StatusInternalServerError, "credentials unavailable", errors.New("credential store not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	credentialID, err := uuid.Parse(chi.URLParam(r, "credentialID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid credential id", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	record, err := h.CredentialStore.GetCredential(ctx, credentialID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			RespondWithError(w, http.StatusNotFound, "credential not found", err)
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to load credential", err)
		return
	}

	if record.CompanyID != session.CompanyID {
		RespondWithError(w, http.StatusNotFound, "credential not found", errors.New("credential scope mismatch"))
		return
	}

	if err := h.CredentialStore.DeleteCredential(ctx, credentialID); err != nil {
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
	if isHTMX(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TestProviderCredential validates a provider credential without persisting it.
func (h *AI) TestProviderCredential(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		RespondWithError(w, http.StatusInternalServerError, "ai system unavailable", errors.New("ai handler not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	providerID := strings.TrimSpace(chi.URLParam(r, "providerID"))

	var req upsertProviderCredentialRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		return
	}
	req.Provider = strings.TrimSpace(req.Provider)
	req.APIKey = strings.TrimSpace(req.APIKey)
	if providerID == "" {
		providerID = req.Provider
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
	}

	if providerID == "" {
		providerID = h.DefaultProvider
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
	if isHTMX(r) {
		w.WriteHeader(http.StatusNoContent)
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
		RespondWithError(w, http.StatusInternalServerError, "credentials unavailable", errors.New("credential store not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	providerID := chi.URLParam(r, "providerID")
	if providerID == "" {
		providerID = h.DefaultProvider
	}

	var req upsertProviderCredentialRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		return
	}
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.APIKey == "" {
		RespondWithError(w, http.StatusBadRequest, "apiKey is required", errors.New("missing api key"))
		return
	}

	if err := validateAPIKeyFormat(providerID, req.APIKey); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid api key", err)
		return
	}

	scopeUser, _, err := resolveCredentialScope(session, req.Scope, req.UserID)
	if err != nil {
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

	labelValue := strings.TrimSpace(req.Label)
	var labelPtr *string
	if labelValue != "" {
		labelPtr = &labelValue
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ciphertext, err := h.CredentialCipher.Encrypt(ctx, []byte(req.APIKey))
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to encrypt credential", err)
		return
	}

	record := ai.CredentialRecord{
		CompanyID:        session.CompanyID,
		UserID:           scopeUser,
		ProviderID:       providerID,
		CredentialCipher: ciphertext,
		CredentialHash:   hashSecret([]byte(req.APIKey)),
		Metadata:         metadata,
		Label:            labelPtr,
		IsDefault:        req.MakeDefault,
	}

	status := http.StatusCreated
	if req.CredentialID != "" {
		credentialID, err := uuid.Parse(req.CredentialID)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "invalid credential id", err)
			return
		}

		existing, err := h.CredentialStore.GetCredential(ctx, credentialID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				RespondWithError(w, http.StatusNotFound, "credential not found", err)
				return
			}
			RespondWithError(w, http.StatusInternalServerError, "failed to load credential", err)
			return
		}

		if existing.CompanyID != session.CompanyID || existing.ProviderID != providerID {
			RespondWithError(w, http.StatusForbidden, "credential scope mismatch", errors.New("credential does not belong to company"))
			return
		}

		if (existing.UserID.Valid && !scopeUser.Valid) || (!existing.UserID.Valid && scopeUser.Valid) || (existing.UserID.Valid && scopeUser.Valid && existing.UserID.UUID != scopeUser.UUID) {
			RespondWithError(w, http.StatusBadRequest, "scope cannot be changed on update", errors.New("scope update not supported"))
			return
		}

		record.ID = credentialID
		record.UserID = existing.UserID
		status = http.StatusOK
	}

	if record.IsDefault {
		if err := h.CredentialStore.ClearDefault(ctx, session.CompanyID, providerID, record.UserID); err != nil {
			RespondWithError(w, http.StatusInternalServerError, "failed to update default credential", err)
			return
		}
	}

	stored, err := h.CredentialStore.UpsertCredential(ctx, record)
	if err != nil {
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
	if isHTMX(r) {
		w.WriteHeader(http.StatusNoContent)
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

func renderCredentialTable(ctx context.Context, w http.ResponseWriter, views []pages.AICredentialView) error {
	var buf bytes.Buffer
	buf.WriteString(`<div id="credential-table">`)
	if err := pages.AICredentialTable(views).Render(ctx, &buf); err != nil {
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
