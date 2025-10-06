package catalog

import "sort"

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
