package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/JonMunkholm/RevProject1/internal/ai"
)

// ListProviderCatalog returns metadata about supported AI providers.
func (h *AI) ListProviderCatalog(w http.ResponseWriter, r *http.Request) {
	entries := h.catalogEntries(r.Context())
	RespondWithJSON(w, http.StatusOK, struct {
		Items []ai.ProviderCatalogEntry `json:"items"`
	}{Items: entries})
}

func (h *AI) catalogEntries(ctx context.Context) []ai.ProviderCatalogEntry {
	if h == nil {
		return ai.ProviderCatalog()
	}
	if h.CatalogLoader != nil {
		if entries := h.CatalogLoader.Entries(ctx); len(entries) > 0 {
			return entries
		}
	}
	if len(h.ProviderCatalog) > 0 {
		return h.ProviderCatalog
	}
	return ai.ProviderCatalog()
}

func (h *AI) catalogEntry(ctx context.Context, providerID string) (ai.ProviderCatalogEntry, bool) {
	id := strings.TrimSpace(providerID)
	if id == "" {
		return ai.ProviderCatalogEntry{}, false
	}
	for _, entry := range h.catalogEntries(ctx) {
		if strings.EqualFold(entry.ID, id) {
			return entry, true
		}
	}
	return ai.ProviderCatalogEntry{}, false
}

func (h *AI) normalizeProvider(ctx context.Context, providerID string) (string, ai.ProviderCatalogEntry, error) {
	id := strings.TrimSpace(providerID)
	if id == "" {
		id = h.DefaultProvider
	}
	entry, ok := h.catalogEntry(ctx, id)
	if !ok {
		return "", ai.ProviderCatalogEntry{}, fmt.Errorf("unknown provider %q", id)
	}
	return entry.ID, entry, nil
}

func (h *AI) pingProvider(ctx context.Context, providerID string, entry ai.ProviderCatalogEntry, apiKey string, metadata map[string]any) error {
	switch providerID {
	case "openai":
		baseURL := metadataString(metadata, "base_url")
		if baseURL == "" {
			baseURL = metadataString(metadata, "baseUrl")
		}
		return pingOpenAI(ctx, baseURL, apiKey)
	default:
		return fmt.Errorf("%w: %s", errStatusNotImplemented, providerID)
	}
}

func pingOpenAI(ctx context.Context, baseURL, apiKey string) error {
	if apiKey == "" {
		return errors.New("missing api key")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	url := baseURL + "/models?limit=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			log.Printf("failed to close response body: %v", cerr)
		}
	}()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("openai status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	if value, ok := meta[key]; ok {
		if str, ok := value.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return ""
}
