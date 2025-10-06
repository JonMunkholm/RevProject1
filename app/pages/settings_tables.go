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
            fingerprint := item.Fingerprint
            if fingerprint == "" {
                fingerprint = "—"
            }

            if _, err := fmt.Fprintf(w,
                `<tr><td>%s</td><td>%s</td><td>%s</td><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td><td>`+
                    `<div class="ai-settings__row-actions">`+
                    `<button class="ai-settings__link" hx-post="/api/ai/providers/%s/credential/test" hx-vals="{&quot;credentialId&quot;:&quot;%s&quot;}" hx-target="#ai-settings-notice" hx-swap="innerHTML">Test</button>`+
                    `<button class="ai-settings__link ai-settings__link--danger" hx-delete="/api/ai/credentials/%s" hx-target="#ai-settings-notice" hx-swap="innerHTML" hx-confirm="Delete this credential?">Delete</button>`+
                    `</div></td></tr>`,
                templ.EscapeString(item.Provider),
                templ.EscapeString(item.ScopeLabel),
                templ.EscapeString(label),
                templ.EscapeString(fingerprint),
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
