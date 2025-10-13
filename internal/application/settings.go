package application

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/ai"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/contextutil"
)

func (a *App) loadSettingsRoutes(r chi.Router) {
	r.Use(auth.RequireCompanyRole(auth.RoleViewer))

	r.Get("/", a.settingsGeneralPage())
	r.Get("/users", a.settingsUsersPage())
	r.Get("/ai", a.settingsAIPage())
}

func (a *App) settingsGeneralPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := auth.SessionFromContext(r.Context())
		if !ok {
			auth.RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
			return
		}

		tabs := a.availableSettingsTabs(r.Context(), session)
		if len(tabs) == 0 {
			auth.RespondWithError(w, http.StatusForbidden, "no accessible settings", errors.New("insufficient role"))
			return
		}

		if !tabActive(tabs, "general") {
			http.Redirect(w, r, tabs[0].Path, http.StatusSeeOther)
			return
		}

		component := pages.SettingsGeneralPage(activateSettingsTabs(tabs, "general"))
		a.render(w, r, component)
	}
}

func (a *App) settingsUsersPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := auth.SessionFromContext(r.Context())
		if !ok {
			auth.RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
			return
		}

		tabs := a.availableSettingsTabs(r.Context(), session)
		if len(tabs) == 0 {
			auth.RespondWithError(w, http.StatusForbidden, "no accessible settings", errors.New("insufficient role"))
			return
		}

		if !session.CurrentRole.Meets(auth.RoleAdmin) {
			message := "You need to be an admin to view user management."
			if isHTMXRequest(r) {
				w.WriteHeader(http.StatusForbidden)
				if err := pages.SettingsWarningContent(message).Render(r.Context(), w); err != nil {
					http.Error(w, "Failed to render", http.StatusInternalServerError)
				}
				return
			}
			component := pages.SettingsWarningPage(tabs, message)
			w.WriteHeader(http.StatusForbidden)
			a.render(w, r, component)
			return
		}

		component := pages.SettingsUsersPage(activateSettingsTabs(tabs, "users"))
		a.render(w, r, component)
	}
}

func (a *App) settingsAIPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := auth.SessionFromContext(r.Context())
		if !ok {
			auth.RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
			return
		}

		ctx := r.Context()
		tabs := a.availableSettingsTabs(ctx, session)
		if len(tabs) == 0 {
			auth.RespondWithError(w, http.StatusForbidden, "no accessible settings", errors.New("insufficient role"))
			return
		}

		if !contextutil.CanViewProviderCredentials(ctx) {
			message := "You do not have permission to view AI provider configuration."
			if isHTMXRequest(r) {
				w.WriteHeader(http.StatusForbidden)
				if err := pages.SettingsWarningContent(message).Render(ctx, w); err != nil {
					http.Error(w, "Failed to render", http.StatusInternalServerError)
				}
				return
			}
			component := pages.SettingsWarningPage(tabs, message)
			w.WriteHeader(http.StatusForbidden)
			a.render(w, r, component)
			return
		}

		providerID := strings.TrimSpace(r.URL.Query().Get("provider"))
		props := a.buildAIProps(ctx, session, providerID)

		if isHTMXRequest(r) {
			if err := pages.SettingsAIContent(props).Render(ctx, w); err != nil {
				http.Error(w, "Failed to render", http.StatusInternalServerError)
			}
			return
		}

		component := pages.SettingsAIPage(activateSettingsTabs(tabs, "ai"), props)
		a.render(w, r, component)
	}
}

func (a *App) availableSettingsTabs(ctx context.Context, session auth.Session) []pages.SettingsTab {
	tabs := make([]pages.SettingsTab, 0, 3)
	if contextutil.CanViewCompanySettings(ctx) {
		tabs = append(tabs, pages.SettingsTab{ID: "general", Label: "General", Path: "/app/settings"})
	}
	if session.CurrentRole.Meets(auth.RoleAdmin) {
		tabs = append(tabs, pages.SettingsTab{ID: "users", Label: "Users", Path: "/app/settings/users"})
	}
	if contextutil.CanViewProviderCredentials(ctx) {
		tabs = append(tabs, pages.SettingsTab{ID: "ai", Label: "AI", Path: "/app/settings/ai"})
	}
	return tabs
}

func activateSettingsTabs(tabs []pages.SettingsTab, active string) []pages.SettingsTab {
	out := make([]pages.SettingsTab, len(tabs))
	for i, tab := range tabs {
		tab.Active = tab.ID == active
		out[i] = tab
	}
	return out
}

func tabActive(tabs []pages.SettingsTab, target string) bool {
	for _, tab := range tabs {
		if tab.ID == target {
			return true
		}
	}
	return false
}

func (a *App) buildAIProps(ctx context.Context, session auth.Session, providerID string) pages.SettingsAIProps {
	catalog := ai.ProviderCatalog()
	if a.providerCatalog != nil {
		if entries := a.providerCatalog.Entries(ctx); len(entries) > 0 {
			catalog = entries
		}
	}
	providers := make([]pages.SettingsAIProvider, 0, len(catalog))
	for _, entry := range catalog {
		fields := make([]pages.SettingsAIField, 0, len(entry.Fields))
		for _, field := range entry.Fields {
			fields = append(fields, pages.SettingsAIField{
				ID:          field.ID,
				Label:       field.Label,
				Type:        field.Type,
				Required:    field.Required,
				Sensitive:   field.Sensitive,
				Placeholder: field.Placeholder,
				Description: field.Description,
				Options:     append([]string(nil), field.Options...),
			})
		}
		providers = append(providers, pages.SettingsAIProvider{
			ID:               entry.ID,
			Label:            entry.Label,
			IconURL:          entry.IconURL,
			Description:      entry.Description,
			DocumentationURL: entry.DocumentationURL,
			Capabilities:     append([]string(nil), entry.Capabilities...),
			Models:           append([]string(nil), entry.Models...),
			Fields:           fields,
		})
	}

	props := pages.SettingsAIProps{
		Providers:          providers,
		ActiveProviderID:   providerID,
		CanManageCompany:   session.Capabilities.CanManageCompanyCredentials,
		CanManagePersonal:  session.Capabilities.CanManagePersonalCredentials,
		CanViewCredentials: session.Capabilities.CanViewProviderCredentials,
		HasProviders:       len(providers) > 0,
	}

	if len(providers) == 0 {
		return props
	}

	activeID := providerID
	if activeID == "" {
		activeID = defaultAIProvider
	}

	var active pages.SettingsAIProvider
	for _, provider := range providers {
		if provider.ID == activeID {
			active = provider
			break
		}
	}
	if active.ID == "" {
		active = providers[0]
		activeID = active.ID
	}

	props.ActiveProvider = active
	props.ActiveProviderID = activeID
	return props
}

func isHTMXRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}
