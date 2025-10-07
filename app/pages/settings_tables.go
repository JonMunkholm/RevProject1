package pages

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"
)

func AICredentialTable(items []AICredentialView) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if len(items) == 0 {
			_, err := io.WriteString(w, `<div class="ai-settings__empty">No credentials configured yet.</div>`)
			return err
		}

		if _, err := io.WriteString(w, `<table class="ai-settings__table"><thead><tr><th>Provider</th><th>Scope</th><th>Label</th><th>Fingerprint</th><th>Metadata</th><th>Updated</th><th>Last used</th><th>Actions</th></tr></thead><tbody>`); err != nil {
			return err
		}

		for _, item := range items {
			label := item.Label
			if label == "" {
				label = "—"
			}

			fingerprintDisplay := item.Fingerprint
			if item.KeySuffix != "" {
				fingerprintDisplay = "..." + item.KeySuffix
			} else if fingerprintDisplay == "" {
				fingerprintDisplay = "—"
			}

			fingerprintAttr := ""
			if item.KeySuffix != "" && item.Fingerprint != "" {
				fingerprintAttr = fmt.Sprintf(" title=\"%s\"", templ.EscapeString(item.Fingerprint))
			}

			if _, err := fmt.Fprintf(w,
				`<tr><td>%s</td><td>%s</td><td>%s</td><td><code%s>%s</code></td><td>%s</td><td>%s</td><td>%s</td><td>`+
					`<div class="ai-settings__row-actions">`+
					`<button class="ai-settings__link" hx-post="/api/ai/providers/%s/credential/test" hx-vals="{&quot;credentialId&quot;:&quot;%s&quot;}" hx-target="#ai-settings-notice" hx-swap="innerHTML">Test</button>`+
					`<button class="ai-settings__link ai-settings__link--danger" hx-delete="/api/ai/credentials/%s" hx-target="#ai-settings-notice" hx-swap="innerHTML" hx-confirm="Delete this credential?">Delete</button>`+
					`</div></td></tr>`,
				templ.EscapeString(item.Provider),
				templ.EscapeString(item.ScopeLabel),
				templ.EscapeString(label),
				fingerprintAttr,
				templ.EscapeString(fingerprintDisplay),
				renderMetadataHTML(item.Metadata),
				templ.EscapeString(item.UpdatedAt.Format(time.RFC822)),
				templ.EscapeString(renderMaybeTimeString(item.LastUsedAt)),
				templ.EscapeString(item.Provider),
				templ.EscapeString(item.ID),
				templ.EscapeString(item.ID),
			); err != nil {
				return err
			}
		}

		_, err := io.WriteString(w, `</tbody></table>`)
		return err
	})
}

func AICredentialEventsTable(events []AICredentialEventView) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if len(events) == 0 {
			_, err := io.WriteString(w, `<div class="ai-settings__empty">No activity yet.</div>`)
			return err
		}

		if _, err := io.WriteString(w, `<table class="ai-settings__table ai-settings__table--events"><thead>`+
			`<tr><th>When</th><th>Action</th><th>Actor</th><th>User</th><th>Metadata</th></tr></thead><tbody>`); err != nil {
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

			if _, err := fmt.Fprintf(w,
				`<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
				templ.EscapeString(event.CreatedAt.Format(time.RFC822)),
				templ.EscapeString(strings.Title(event.Action)),
				templ.EscapeString(actor),
				templ.EscapeString(user),
				renderMetadataHTML(event.Metadata),
			); err != nil {
				return err
			}
		}

		_, err := io.WriteString(w, `</tbody></table>`)
		return err
	})
}

func renderMetadataHTML(meta map[string]any) string {
	if len(meta) == 0 {
		return "<span>—</span>"
	}
	keys := make([]string, 0, len(meta))
	for key := range meta {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	builder.WriteString(`<dl class="ai-settings__meta">`)
	for _, key := range keys {
		builder.WriteString(`<div><dt>`)
		builder.WriteString(templ.EscapeString(key))
		builder.WriteString(`</dt><dd>`)
		builder.WriteString(templ.EscapeString(fmt.Sprintf("%v", meta[key])))
		builder.WriteString(`</dd></div>`)
	}
	builder.WriteString(`</dl>`)
	return builder.String()
}

func renderMaybeTimeString(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Format(time.RFC822)
}

func ProviderClasses(active bool) string {
	classes := []string{"ai-settings__provider"}
	if active {
		classes = append(classes, "ai-settings__provider--active")
	}
	return strings.Join(classes, " ")
}

func ProviderAriaCurrent(active bool) string {
	if active {
		return "page"
	}
	return ""
}

func CompanyScopeClasses(canManage bool) string {
	if canManage {
		return ""
	}
	return "ai-settings__option--disabled"
}

func ProviderFieldID(field SettingsAIField) string {
	return fmt.Sprintf("provider-field-%s", field.ID)
}

func ProviderFieldType(field SettingsAIField) string {
	if field.Type == "" {
		return "text"
	}
	return field.Type
}

func ProviderFieldAutoComplete(field SettingsAIField) string {
	if field.Sensitive {
		return "off"
	}
	return "on"
}

func NoticeClasses(status string) string {
	classes := []string{"ai-settings__notice"}
	switch status {
	case "success":
		classes = append(classes, "ai-settings__notice--success")
	case "warning":
		classes = append(classes, "ai-settings__notice--warning")
	case "error":
		classes = append(classes, "ai-settings__notice--error")
	default:
		classes = append(classes, "ai-settings__notice--info")
	}
	return strings.Join(classes, " ")
}

func StatusBadgeClasses(status string) string {
	classes := []string{"status-badge"}
	switch status {
	case "ok":
		classes = append(classes, "status-badge--ok")
	case "error":
		classes = append(classes, "status-badge--error")
	case "warning":
		classes = append(classes, "status-badge--warning")
	default:
		classes = append(classes, "status-badge--loading")
	}
	return strings.Join(classes, " ")
}

func SettingsTabClass(active bool) string {
	classes := []string{"settings-tabs__item"}
	if active {
		classes = append(classes, "settings-tabs__item--active")
	}
	return strings.Join(classes, " ")
}
