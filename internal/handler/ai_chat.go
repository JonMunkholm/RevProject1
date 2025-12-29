package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/ai/conversation"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/config"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

// ChatCreateSession starts a new conversation and returns the chat shell markup for HTMX consumers.
func (h *AI) ChatCreateSession(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Conversations == nil {
		h.writeChatShell(w, r.Context(), pages.ChatPageProps{ErrorMessage: "AI conversations unavailable."})
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		h.writeChatShell(w, r.Context(), pages.ChatPageProps{ErrorMessage: "Authentication required."})
		return
	}

	var req createConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		h.writeChatShell(w, r.Context(), pages.ChatPageProps{ErrorMessage: "Invalid request payload."})
		return
	}

	props, err := h.BuildChatProps(r.Context(), session, req.Provider, "", true)
	if err != nil {
		log.Printf("chat: failed to build chat props: %v", err)
		props.ErrorMessage = "Failed to start conversation. Please try again."
	}

	h.writeChatShell(w, r.Context(), props)
}

// ChatLoadSession renders the chat shell for an existing conversation.
func (h *AI) ChatLoadSession(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Conversations == nil {
		h.writeChatShell(w, r.Context(), pages.ChatPageProps{ErrorMessage: "AI conversations unavailable."})
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		h.writeChatShell(w, r.Context(), pages.ChatPageProps{ErrorMessage: "Authentication required."})
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		h.writeChatShell(w, r.Context(), pages.ChatPageProps{ErrorMessage: "Invalid conversation."})
		return
	}

	providerCandidate := strings.TrimSpace(r.URL.Query().Get("provider"))
	props, err := h.BuildChatProps(r.Context(), session, providerCandidate, sessionID.String(), false)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fallback, buildErr := h.BuildChatProps(r.Context(), session, providerCandidate, "", true)
			if buildErr != nil {
				log.Printf("chat: failed to build fallback chat props: %v", buildErr)
				h.writeChatShell(w, r.Context(), pages.ChatPageProps{ErrorMessage: "Conversation not found. Start a new conversation."})
				return
			}
			fallback.ErrorMessage = "Conversation not found. Start a new conversation."
			h.writeChatShell(w, r.Context(), fallback)
			return
		}

		log.Printf("chat: failed to load conversation %s: %v", sessionID, err)
		h.writeChatShell(w, r.Context(), pages.ChatPageProps{ErrorMessage: "Failed to load conversation."})
		return
	}

	h.writeChatShell(w, r.Context(), props)
}

// ChatListSessions returns additional conversation list items for infinite scroll.
func (h *AI) ChatListSessions(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Conversations == nil {
		http.Error(w, "AI conversations unavailable.", http.StatusServiceUnavailable)
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		http.Error(w, "Authentication required.", http.StatusUnauthorized)
		return
	}

	offsetParam := strings.TrimSpace(r.URL.Query().Get("offset"))
	offset := 0
	if offsetParam != "" {
		if parsed, err := strconv.Atoi(offsetParam); err == nil && parsed > 0 {
			offset = parsed
		}
	}

	activeCandidate := strings.TrimSpace(r.URL.Query().Get("active"))
	var activeID uuid.UUID
	if activeCandidate != "" {
		if parsed, err := uuid.Parse(activeCandidate); err == nil {
			activeID = parsed
		}
	}

	providerCandidate := strings.TrimSpace(r.URL.Query().Get("provider"))

	entries := h.catalogEntries(r.Context())
	providerLookup := make(map[string]ai.ProviderCatalogEntry, len(entries))
	for _, entry := range entries {
		providerLookup[entry.ID] = entry
	}

	ctxList, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	sessions, err := h.Conversations.ListCompanySessions(ctxList, session.CompanyID, config.DefaultConversationLimit, int32(offset))
	cancel()
	if err != nil {
		log.Printf("chat: failed to list conversations: %v", err)
		http.Error(w, "Failed to load conversations.", http.StatusInternalServerError)
		return
	}

	views := chatSessionsToView(sessions, providerLookup, activeID)
	hasMore := len(sessions) == int(config.DefaultConversationLimit)
	nextOffset := offset + len(sessions)

	props := pages.ChatConversationChunkProps{
		Conversations:        views,
		NextOffset:           nextOffset,
		HasMore:              hasMore,
		ActiveConversationID: activeCandidate,
		ActiveProviderID:     providerCandidate,
	}

	h.writeChatConversationItems(w, r.Context(), props)
}

// ChatAppendMessage appends a message to an existing conversation and renders the updated transcript.
func (h *AI) ChatAppendMessage(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Conversations == nil {
		h.writeChatTranscript(w, r.Context(), pages.ChatTranscriptProps{ErrorMessage: "AI conversations unavailable."})
		return
	}

	sessionInfo, ok := auth.SessionFromContext(r.Context())
	if !ok {
		h.writeChatTranscript(w, r.Context(), pages.ChatTranscriptProps{ErrorMessage: "Authentication required."})
		return
	}

	sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		h.writeChatTranscript(w, r.Context(), pages.ChatTranscriptProps{ErrorMessage: "Invalid conversation."})
		return
	}

	var req appendMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		h.writeChatTranscript(w, r.Context(), pages.ChatTranscriptProps{ConversationID: sessionID.String(), ErrorMessage: "Invalid request payload.", ProviderID: strings.TrimSpace(req.Provider)})
		return
	}
	providerHint := strings.TrimSpace(req.Provider)

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "user"
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		h.writeChatTranscript(w, r.Context(), pages.ChatTranscriptProps{
			ConversationID: sessionID.String(),
			ErrorMessage:   "Message content is required.",
			ProviderID:     providerHint,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	sessionRecord, messages, _, appendErr := h.appendConversationAndListMessages(ctx, sessionInfo, sessionID, providerHint, role, content, req.Metadata)
	if appendErr != nil {
		props := pages.ChatTranscriptProps{ConversationID: sessionID.String(), ErrorMessage: "Failed to send message."}
		if errors.Is(appendErr, sql.ErrNoRows) {
			props.ErrorMessage = "Conversation not found. Start a new conversation."
		}
		providerID := sessionRecord.ProviderID
		if providerID == "" {
			providerID = providerHint
		}
		if blocked := h.chatCredentialReason(ctx, sessionInfo, providerID); blocked != "" {
			props.BlockedReason = blocked
		}
		props.ProviderID = providerID
		h.writeChatTranscript(w, r.Context(), props)
		return
	}

	props := pages.ChatTranscriptProps{
		ConversationID: sessionRecord.ID.String(),
		Messages:       chatMessagesToView(messages),
		ProviderID:     sessionRecord.ProviderID,
	}
	h.writeChatTranscript(w, r.Context(), props)
}

// BuildChatProps constructs the props for rendering the chat page.
func (h *AI) BuildChatProps(ctx context.Context, session auth.Session, providerCandidate, conversationCandidate string, ensureSession bool) (pages.ChatPageProps, error) {
	props := pages.ChatPageProps{}
	entries := h.catalogEntries(ctx)
	props.Providers = chatProvidersFromEntries(entries)

	if len(entries) == 0 {
		props.BlockedReason = "No providers available. Add a credential in Settings → AI."
		return props, nil
	}

	providerLookup := make(map[string]ai.ProviderCatalogEntry, len(entries))
	for _, entry := range entries {
		providerLookup[entry.ID] = entry
	}

	activeID := strings.TrimSpace(providerCandidate)
	if activeID == "" {
		activeID = h.DefaultProvider
	}

	activeEntry, found := providerLookup[activeID]
	if !found {
		activeEntry = entries[0]
		activeID = activeEntry.ID
	}

	props.ActiveProviderID = activeID
	props.ActiveProviderLabel = activeEntry.Label
	props.BlockedReason = h.chatCredentialReason(ctx, session, activeID)

	if h.Conversations == nil {
		return props, errors.New("conversation service not configured")
	}

	var (
		sessionRecord conversation.Session
		messages      []conversation.Message
		hasSession    bool
		err           error
	)

	if conversationCandidate != "" {
		conversationID, err := uuid.Parse(conversationCandidate)
		if err != nil {
			if ensureSession {
				props.ErrorMessage = "Invalid conversation reference. Starting a new conversation."
			} else {
				return props, sql.ErrNoRows
			}
		} else {
			ctxLookup, cancel := context.WithTimeout(ctx, 10*time.Second)
			sessionRecord, err = h.Conversations.Session(ctxLookup, session.CompanyID, conversationID)
			cancel()
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					if ensureSession {
						props.ErrorMessage = "Conversation not found. Starting a new conversation."
					} else {
						return props, err
					}
				} else {
					return props, err
				}
			} else {
				activeID = sessionRecord.ProviderID
				hasSession = true
				if entry, ok := providerLookup[activeID]; ok {
					activeEntry = entry
				} else {
					activeEntry = ai.ProviderCatalogEntry{ID: activeID, Label: activeID}
				}
				props.ActiveProviderID = activeEntry.ID
				props.ActiveProviderLabel = activeEntry.Label
				props.BlockedReason = h.chatCredentialReason(ctx, session, activeID)
			}
		}
	}

	var conversationID uuid.UUID
	if !hasSession {
		if !ensureSession {
			return props, sql.ErrNoRows
		}

		conversationID = uuid.New()
		props.ConversationID = conversationID.String()
	} else {
		conversationID = sessionRecord.ID

		ctxMsgs, cancel := context.WithTimeout(ctx, 10*time.Second)
		messages, err = h.Conversations.ListSessionMessages(ctxMsgs, sessionRecord.ID)
		cancel()
		if err != nil {
			return props, err
		}

		props.ConversationID = sessionRecord.ID.String()
		props.Messages = chatMessagesToView(messages)
	}

	ctxList, cancel := context.WithTimeout(ctx, 10*time.Second)
	sessions, err := h.Conversations.ListCompanySessions(ctxList, session.CompanyID, config.DefaultConversationLimit, 0)
	cancel()
	if err != nil {
		return props, err
	}

	var activeConversation uuid.UUID
	if hasSession {
		activeConversation = sessionRecord.ID
	}

	props.Conversations = chatSessionsToView(sessions, providerLookup, activeConversation)
	props.ConversationsNextOffset = len(sessions)
	if len(sessions) == int(config.DefaultConversationLimit) {
		props.ConversationsHasMore = true
	}

	return props, nil
}

// chatCredentialReason checks if credentials are available for the given provider.
func (h *AI) chatCredentialReason(ctx context.Context, session auth.Session, providerID string) string {
	if h.APIKey != "" {
		return ""
	}
	if h.CredentialStore == nil {
		return "Credential store unavailable."
	}

	ctxLookup, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	userScope := uuid.NullUUID{}
	if session.UserID != uuid.Nil {
		userScope = uuid.NullUUID{UUID: session.UserID, Valid: true}
	}

	_, err := h.CredentialStore.ResolveCredential(ctxLookup, session.CompanyID, userScope, providerID)
	if err == nil {
		return ""
	}
	if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("chat: resolve provider credential failed: %v", err)
		return "Unable to verify credentials right now. Try again later."
	}

	return "Add a credential for this provider in Settings → AI before chatting."
}

func chatProvidersFromEntries(entries []ai.ProviderCatalogEntry) []pages.ChatProvider {
	providers := make([]pages.ChatProvider, 0, len(entries))
	for _, entry := range entries {
		providers = append(providers, pages.ChatProvider{ID: entry.ID, Label: entry.Label})
	}
	return providers
}

func chatMessagesToView(messages []conversation.Message) []pages.ChatMessageView {
	views := make([]pages.ChatMessageView, 0, len(messages))
	for _, msg := range messages {
		views = append(views, pages.ChatMessageView{
			ID:        msg.ID.String(),
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		})
	}
	return views
}

func chatSessionsToView(sessions []conversation.Session, providers map[string]ai.ProviderCatalogEntry, active uuid.UUID) []pages.ChatConversationView {
	views := make([]pages.ChatConversationView, 0, len(sessions))
	for _, sessionRecord := range sessions {
		providerLabel := providerLabelForID(providers, sessionRecord.ProviderID)
		title := conversationTitle(sessionRecord, providerLabel)
		preview := conversationPreview(sessionRecord, title)

		views = append(views, pages.ChatConversationView{
			ID:            sessionRecord.ID.String(),
			Title:         title,
			Preview:       preview,
			ProviderID:    sessionRecord.ProviderID,
			ProviderLabel: providerLabel,
			UpdatedAt:     sessionRecord.UpdatedAt,
			IsActive:      sessionRecord.ID == active,
		})
	}
	return views
}

func providerLabelForID(providers map[string]ai.ProviderCatalogEntry, providerID string) string {
	if entry, ok := providers[providerID]; ok {
		return entry.Label
	}
	return providerID
}

func conversationTitle(record conversation.Session, providerLabel string) string {
	if title := strings.TrimSpace(record.Title); title != "" {
		return title
	}
	if providerLabel != "" {
		return fmt.Sprintf("%s conversation", providerLabel)
	}
	return "Conversation"
}

func conversationPreview(record conversation.Session, fallback string) string {
	if record.Metadata != nil {
		for _, key := range []string{"preview", "last_message", "summary"} {
			if raw, ok := record.Metadata[key].(string); ok {
				if trimmed := strings.TrimSpace(raw); trimmed != "" {
					return truncateText(trimmed, config.ChatPreviewCharacterLimit)
				}
			}
		}
	}
	return truncateText(fallback, config.ChatPreviewCharacterLimit)
}

func truncateText(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func (h *AI) writeChatShell(w http.ResponseWriter, ctx context.Context, props pages.ChatPageProps) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.ChatShell(props).Render(ctx, w); err != nil {
		http.Error(w, "Failed to render chat", http.StatusInternalServerError)
	}
}

func (h *AI) writeChatTranscript(w http.ResponseWriter, ctx context.Context, props pages.ChatTranscriptProps) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.ChatTranscript(props).Render(ctx, w); err != nil {
		http.Error(w, "Failed to render chat", http.StatusInternalServerError)
	}
}

func (h *AI) writeChatConversationItems(w http.ResponseWriter, ctx context.Context, props pages.ChatConversationChunkProps) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.ChatConversationItems(props).Render(ctx, w); err != nil {
		http.Error(w, "Failed to render conversations", http.StatusInternalServerError)
	}
}
