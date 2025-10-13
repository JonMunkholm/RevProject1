package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
)

// Field describes a provider-specific form input for credential configuration.
type Field struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Sensitive   bool     `json:"sensitive"`
	Placeholder string   `json:"placeholder,omitempty"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// Entry captures metadata about an AI provider option shown in the catalog.
type Entry struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	IconURL          string   `json:"iconUrl,omitempty"`
	Description      string   `json:"description,omitempty"`
	DocumentationURL string   `json:"documentationUrl,omitempty"`
	Capabilities     []string `json:"capabilities,omitempty"`
	Models           []string `json:"models,omitempty"`
	Fields           []Field  `json:"fields,omitempty"`
}

var defaultEntries = []Entry{
	{
		ID:               "openai",
		Label:            "OpenAI",
		IconURL:          "https://static.openai.com/logo.png",
		Description:      "ChatGPT, GPT-4o, embeddings, and more",
		DocumentationURL: "https://platform.openai.com/docs",
		Capabilities:     []string{"chat", "completion", "embeddings"},
		Models:           []string{"gpt-4o", "gpt-4o-mini", "gpt-3.5-turbo", "text-embedding-3-large"},
		Fields: []Field{
			{ID: "apiKey", Label: "API Key", Type: "password", Required: true, Sensitive: true, Placeholder: "sk-..."},
			{ID: "baseUrl", Label: "Base URL", Type: "url", Placeholder: "https://api.openai.com/v1"},
			{ID: "model", Label: "Default Model", Type: "text", Placeholder: "gpt-4o-mini"},
		},
	},
}

// Catalog returns the default catalog entries in stable order.
func Catalog() []Entry {
	items := make([]Entry, len(defaultEntries))
	copy(items, defaultEntries)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

// Lookup returns the catalog entry for the given provider identifier if present.
func Lookup(id string) (Entry, bool) {
	for _, entry := range defaultEntries {
		if entry.ID == id {
			return entry, true
		}
	}
	return Entry{}, false
}

// Store exposes the subset of database.Queries needed for catalog loading.
type Store interface {
	ListAIProviderCatalogEntries(ctx context.Context) ([]database.AiProviderCatalog, error)
}

// Loader retrieves provider catalog entries from a backing store with optional caching.
type Loader struct {
	store   Store
	ttl     time.Duration
	mu      sync.RWMutex
	entries []Entry
	expires time.Time
}

// NewLoader constructs a Loader that refreshes from the provided Store at most every ttl.
// A non-positive ttl disables caching (entries are fetched on every call).
func NewLoader(store Store, ttl time.Duration) *Loader {
	return &Loader{store: store, ttl: ttl}
}

// Entries returns catalog entries fetched from the store or falls back to the static defaults.
func (l *Loader) Entries(ctx context.Context) []Entry {
	if l == nil || l.store == nil {
		return Catalog()
	}

	now := time.Now()
	if l.ttl > 0 {
		l.mu.RLock()
		if len(l.entries) > 0 && now.Before(l.expires) {
			result := copyEntries(l.entries)
			l.mu.RUnlock()
			return result
		}
		l.mu.RUnlock()
	}

	entries, err := loadEntries(ctx, l.store)
	if err != nil || len(entries) == 0 {
		return Catalog()
	}

	if l.ttl > 0 {
		l.mu.Lock()
		l.entries = entries
		l.expires = now.Add(l.ttl)
		l.mu.Unlock()
	}

	return copyEntries(entries)
}

func loadEntries(ctx context.Context, store Store) ([]Entry, error) {
	rows, err := store.ListAIProviderCatalogEntries(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		entry, err := mapRow(row)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

func mapRow(row database.AiProviderCatalog) (Entry, error) {
	var fields []Field
	if len(row.Fields) > 0 {
		if err := json.Unmarshal(row.Fields, &fields); err != nil {
			return Entry{}, err
		}
	}

	entry := Entry{
		ID:               row.ID,
		Label:            row.Label,
		IconURL:          stringFromNull(row.IconUrl),
		Description:      stringFromNull(row.Description),
		DocumentationURL: stringFromNull(row.DocumentationUrl),
		Capabilities:     append([]string(nil), row.Capabilities...),
		Models:           append([]string(nil), row.Models...),
		Fields:           fields,
	}
	return entry, nil
}

func copyEntries(src []Entry) []Entry {
	out := make([]Entry, len(src))
	for i, entry := range src {
		copyEntry := entry
		copyEntry.Capabilities = append([]string(nil), entry.Capabilities...)
		copyEntry.Models = append([]string(nil), entry.Models...)
		fieldsCopy := make([]Field, len(entry.Fields))
		for j, field := range entry.Fields {
			fieldCopy := field
			fieldCopy.Options = append([]string(nil), field.Options...)
			fieldsCopy[j] = fieldCopy
		}
		copyEntry.Fields = fieldsCopy
		out[i] = copyEntry
	}
	return out
}

func stringFromNull(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
