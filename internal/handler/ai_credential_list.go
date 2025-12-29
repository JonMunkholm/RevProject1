package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/config"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

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

	limit, offset := paginationParams(r, config.DefaultCredentialLimit)
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

	ctx, cancel := context.WithTimeout(r.Context(), config.DefaultHandlerTimeout)
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

	limit, offset := paginationParams(r, config.DefaultCredentialEventLimit)
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

	ctx, cancel := context.WithTimeout(r.Context(), config.DefaultHandlerTimeout)
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
