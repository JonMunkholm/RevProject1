package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/ai/conversation"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/config"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

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

	limit, offset := paginationParams(r, config.DefaultConversationLimit)

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
	providerHint := strings.TrimSpace(req.Provider)
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

	_, _, reply, err := h.appendConversationAndListMessages(ctx, sessionInfo, sessionID, providerHint, role, req.Content, req.Metadata)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			RespondWithError(w, http.StatusNotFound, "conversation not found", err)
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to generate reply", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, struct {
		Message messageResponse `json:"message"`
	}{Message: messageToResponse(reply)})
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

func (h *AI) appendConversationAndListMessages(ctx context.Context, session auth.Session, sessionID uuid.UUID, providerCandidate, role, content string, metadata map[string]any) (conversation.Session, []conversation.Message, conversation.Message, error) {
	providerID := strings.TrimSpace(providerCandidate)
	if providerID == "" {
		providerID = h.DefaultProvider
	}

	sessionRecord, err := h.Conversations.Session(ctx, session.CompanyID, sessionID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return conversation.Session{}, nil, conversation.Message{}, err
		}
		sessionRecord, err = h.createChatSession(ctx, session, sessionID, providerID)
		if err != nil {
			return conversation.Session{}, nil, conversation.Message{}, err
		}
	}
	providerID = sessionRecord.ProviderID

	msg, err := h.Conversations.AppendMessage(ctx, conversation.CreateMessageParams{
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Metadata:  metadata,
	})
	if err != nil {
		return conversation.Session{}, nil, conversation.Message{}, err
	}

	// Update conversation title on first user message
	if role == "user" && strings.HasSuffix(sessionRecord.Title, " conversation") {
		newTitle := generateConversationTitle(content)
		if newTitle != "" {
			_ = h.Conversations.RenameSession(ctx, session.CompanyID, sessionID, newTitle)
		}
	}

	if h.Client == nil {
		messages, listErr := h.Conversations.ListSessionMessages(ctx, sessionID)
		return sessionRecord, messages, msg, listErr
	}

	messages, err := h.Conversations.ListSessionMessages(ctx, sessionID)
	if err != nil {
		return conversation.Session{}, nil, conversation.Message{}, err
	}

	metadataMerged := mergeMetadataMaps(sessionRecord.Metadata, metadata)
	completionMetadata := map[string]any{}
	if addendum, ok := metadataMerged["system_addendum"].(string); ok && addendum != "" {
		completionMetadata = ai.WithSystemAddendum(completionMetadata, addendum)
	}

	options := h.userOptions(ctx, session.CompanyID, session.UserID, sessionRecord.ProviderID)
	if options.APIKey == "" {
		return sessionRecord, messages, msg, errors.New("credential missing for provider")
	}

	prompt := buildConversationPrompt(messages)
	resp, err := h.Client.Completion(ctx, options, ai.CompletionRequest{Prompt: prompt, Metadata: completionMetadata})
	if err != nil {
		return conversation.Session{}, nil, conversation.Message{}, err
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
		return conversation.Session{}, nil, conversation.Message{}, err
	}

	updated, err := h.Conversations.ListSessionMessages(ctx, sessionID)
	if err != nil {
		return conversation.Session{}, nil, conversation.Message{}, err
	}

	return sessionRecord, updated, reply, nil
}

func (h *AI) createChatSession(ctx context.Context, session auth.Session, sessionID uuid.UUID, providerID string) (conversation.Session, error) {
	if sessionID == uuid.Nil {
		return conversation.Session{}, errors.New("conversation id required")
	}

	entries := h.catalogEntries(ctx)
	providerLabel := providerID
	for _, entry := range entries {
		if entry.ID == providerID {
			providerLabel = entry.Label
			break
		}
	}

	ctxCreate, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return h.Conversations.StartSession(ctxCreate, conversation.CreateSessionParams{
		ID:         sessionID,
		CompanyID:  session.CompanyID,
		UserID:     session.UserID,
		ProviderID: providerID,
		Title:      fmt.Sprintf("%s conversation", providerLabel),
	})
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

// generateConversationTitle creates a title from the first user message.
func generateConversationTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	// Take first line or first 60 characters
	if idx := strings.IndexAny(content, "\n\r"); idx > 0 {
		content = content[:idx]
	}

	runes := []rune(content)
	if len(runes) > 60 {
		content = string(runes[:57]) + "..."
	}

	return content
}
