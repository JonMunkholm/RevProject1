package pages

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/app/layout"
	"github.com/a-h/templ"
)

type AICredentialView struct {
	ID          string
	Provider    string
	Scope       string
	ScopeLabel  string
	UserID      *string
	Label       string
	Fingerprint string
	IsDefault   bool
	Metadata    map[string]any
	UpdatedAt   time.Time
	LastUsedAt  *time.Time
	RotatedAt   *time.Time
}

type AICredentialEventView struct {
	ID        string
	Action    string
	ActorID   *string
	UserID    *string
	Metadata  map[string]any
	CreatedAt time.Time
}

func AISettingsPage() templ.Component {
	return layout.LayoutWithAssets(
		"AI Settings",
		[]string{"/assets/css/settings.css"},
		templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			if err := writeString(w, `<section class="ai-settings" hx-ext="json-enc" data-ai-settings>`); err != nil {
				return err
			}
			if err := writeString(w, `<header class="ai-settings__header"><h1>AI Provider Credentials</h1><p>Manage provider API keys for this workspace. Keys are encrypted at rest. Use company scope to share across users or user scope for personal keys.</p></header>`); err != nil {
				return err
			}
			if err := settingsForm().Render(ctx, w); err != nil {
				return err
			}
			if err := writeString(w, `<section class="ai-settings__section"><div id="credential-table" hx-get="/api/ai/providers?limit=20" hx-trigger="load, ai-credentials-refresh from:body" hx-swap="outerHTML"><div class="ai-settings__placeholder">Loading credentials…</div></div></section>`); err != nil {
				return err
			}
			if err := writeString(w, `<section class="ai-settings__section"><h2>Credential Activity</h2><div id="credential-events" class="ai-settings__events" hx-get="/api/ai/providers/events?limit=20" hx-trigger="load, ai-credentials-refresh from:body" hx-swap="outerHTML"><div class="ai-settings__placeholder">Loading activity…</div></div></section>`); err != nil {
				return err
			}
			if err := writeString(w, `</section><script src="https://unpkg.com/htmx.org/dist/ext/json-enc.js" defer></script>`); err != nil {
				return err
			}
			return nil
		}),
	)
}

func settingsForm() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := writeString(w, `<section class="ai-settings__section"><h2>Add or Update Credential</h2><form class="ai-settings__form" hx-post="/api/ai/providers" hx-swap="none">`); err != nil {
			return err
		}
		if err := writeString(w, `<div class="ai-settings__field"><label for="provider">Provider</label><select id="provider" name="provider" required><option value="openai">OpenAI</option></select></div>`); err != nil {
			return err
		}
		if err := writeString(w, `<fieldset class="ai-settings__field"><legend>Scope</legend><label><input type="radio" name="scope" value="user" checked> My account</label><label><input type="radio" name="scope" value="company"> Entire company</label></fieldset>`); err != nil {
			return err
		}
		if err := writeString(w, `<div class="ai-settings__field"><label for="apiKey">API Key</label><input id="apiKey" name="apiKey" type="password" autocomplete="off" required placeholder="sk-..."></div>`); err != nil {
			return err
		}
		if err := writeString(w, `<div class="ai-settings__field"><label for="label">Label (optional)</label><input id="label" name="label" type="text" placeholder="Production key"></div>`); err != nil {
			return err
		}
		if err := writeString(w, `<div class="ai-settings__field"><label for="model">Model (optional)</label><input id="model" name="model" type="text" placeholder="gpt-4o-mini"></div>`); err != nil {
			return err
		}
		if err := writeString(w, `<div class="ai-settings__field"><label for="baseUrl">Base URL (optional)</label><input id="baseUrl" name="baseUrl" type="url" placeholder="https://api.openai.com/v1"></div>`); err != nil {
			return err
		}
		if err := writeString(w, `<div class="ai-settings__field ai-settings__field--inline"><label><input type="checkbox" name="makeDefault"> Make default for this scope</label></div>`); err != nil {
			return err
		}
		if err := writeString(w, `<div class="ai-settings__actions"><button type="submit" class="ai-settings__button">Save Credential</button><button type="button" class="ai-settings__button ai-settings__button--secondary" hx-post="/api/ai/providers/test" hx-include="closest form" hx-swap="none">Test</button></div>`); err != nil {
			return err
		}
		return writeString(w, `</form></section>`)
	})
}

func AICredentialTable(items []AICredentialView) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if len(items) == 0 {
			return writeString(w, `<div class="ai-settings__empty">No credentials configured yet.</div>`)
		}
		if err := writeString(w, `<table class="ai-settings__table"><thead><tr><th>Provider</th><th>Scope</th><th>Label</th><th>Fingerprint</th><th>Metadata</th><th>Updated</th><th>Last Used</th><th>Actions</th></tr></thead><tbody>`); err != nil {
			return err
		}
		for _, item := range items {
			label := item.Label
			if label == "" {
				label = "—"
			}
			fingerprint := item.Fingerprint
			if fingerprint == "" {
				fingerprint = "—"
			}
			if err := writeString(w, `<tr><td>`+templ.EscapeString(item.Provider)+`</td><td>`+templ.EscapeString(item.ScopeLabel)+`</td><td>`+templ.EscapeString(label)+`</td><td><code>`+templ.EscapeString(fingerprint)+`</code></td><td>`+renderMetadata(item.Metadata)+`</td><td>`+templ.EscapeString(item.UpdatedAt.Format(time.RFC822))+`</td><td>`+renderTime(item.LastUsedAt)+`</td><td>`); err != nil {
				return err
			}
			hxVals := fmt.Sprintf(`{"credentialId":"%s","provider":"%s"}`, item.ID, item.Provider)
			deleteHref := "/api/ai/credentials/" + templ.EscapeString(item.ID)
			if err := writeString(w, `<div class="ai-settings__row-actions"><button class="ai-settings__link" hx-post="/api/ai/providers/test" hx-vals='`+hxVals+`' hx-swap="none">Test</button><button class="ai-settings__link ai-settings__link--danger" hx-delete="`+deleteHref+`" hx-confirm="Delete this credential?" hx-swap="none">Delete</button></div>`); err != nil {
				return err
			}
			if err := writeString(w, `</td></tr>`); err != nil {
				return err
			}
		}
		return writeString(w, `</tbody></table>`)
	})
}

func AICredentialEventsTable(events []AICredentialEventView) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if len(events) == 0 {
			return writeString(w, `<div class="ai-settings__empty">No activity yet.</div>`)
		}
		if err := writeString(w, `<table class="ai-settings__table ai-settings__table--events"><thead><tr><th>When</th><th>Action</th><th>Actor</th><th>User</th><th>Metadata</th></tr></thead><tbody>`); err != nil {
			return err
		}
		for _, event := range events {
			actor := "—"
			if event.ActorID != nil && *event.ActorID != "" {
				actor = *event.ActorID
			}
			user := "—"
			if event.UserID != nil && *event.UserID != "" {
				user = *event.UserID
			}
			if err := writeString(w, `<tr><td>`+templ.EscapeString(event.CreatedAt.Format(time.RFC822))+`</td><td>`+templ.EscapeString(strings.Title(event.Action))+`</td><td>`+templ.EscapeString(actor)+`</td><td>`+templ.EscapeString(user)+`</td><td>`+renderMetadata(event.Metadata)+`</td></tr>`); err != nil {
				return err
			}
		}
		return writeString(w, `</tbody></table>`)
	})
}

func renderMetadata(meta map[string]any) string {
	if len(meta) == 0 {
		return "—"
	}
	builder := strings.Builder{}
	builder.WriteString(`<dl class="ai-settings__meta">`)
	for key, value := range meta {
		builder.WriteString(`<div><dt>` + templ.EscapeString(key) + `</dt><dd>` + templ.EscapeString(fmt.Sprintf("%v", value)) + `</dd></div>`)
	}
	builder.WriteString(`</dl>`)
	return builder.String()
}

func renderTime(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return templ.EscapeString(t.Format(time.RFC822))
}

func writeString(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
