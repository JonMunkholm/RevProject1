package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/config"
	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/google/uuid"
)

// Response converters

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

// Scope and permission helpers

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

// Request parsing

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

// Validation helpers

func validateAPIKeyFormat(providerID, key string) error {
	switch providerID {
	case "openai":
		if !strings.HasPrefix(key, "sk-") {
			return fmt.Errorf("openai api keys must begin with 'sk-'")
		}
	}
	return nil
}

// Credential data helpers

func hashSecret(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
}

func deriveCredentialSuffix(_ string, apiKey string) string {
	trimmed := strings.TrimSpace(apiKey)
	if len(trimmed) < 4 {
		return ""
	}
	return trimmed[len(trimmed)-4:]
}

func credentialSuffixFromMetadata(metadata map[string]any) (string, bool) {
	if metadata == nil {
		return "", false
	}
	value, ok := metadata[config.MetadataKeyCredentialSuffix]
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

// Rendering helpers

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

// HTMX response helpers

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

// Event recording

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
	ctx, cancel := context.WithTimeout(ctx, config.ShortTimeout)
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
