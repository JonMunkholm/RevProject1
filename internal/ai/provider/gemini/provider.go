package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	clientpkg "github.com/JonMunkholm/RevProject1/internal/ai/client"
	"github.com/JonMunkholm/RevProject1/internal/ai/tool"
)

var (
	ErrMissingAPIKey = errors.New("ai: gemini api key not provided")
	defaultBaseURL   = "https://generativelanguage.googleapis.com/v1beta"
)

// Config captures optional Gemini provider settings.
type Config struct {
	BaseURL string
	Model   string
	Logger  clientpkg.Logger
}

// Provider implements the client.Provider interface for Gemini APIs.
type Provider struct {
	httpClient *http.Client
	executor   *tool.Executor
	logger     clientpkg.Logger
	config     Config
	apiKey     string
	model      string
	baseURL    string
	metadata   map[string]any
}

type Option func(*Provider)

// Factory constructs a Gemini provider compatible with the AI client.
func Factory(cfg Config) clientpkg.ProviderFactory {
	return func(init clientpkg.ProviderInit) (clientpkg.Provider, error) {
		if strings.TrimSpace(init.APIKey) == "" {
			return nil, ErrMissingAPIKey
		}

		httpClient := init.HTTPClient
		if httpClient == nil {
			httpClient = http.DefaultClient
		}

		logger := cfg.Logger
		if logger == nil {
			logger = clientpkg.NewNoopLogger()
		}

		model := cfg.Model
		if model == "" {
			model = "gemini-pro"
		}

		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = defaultBaseURL
		}

		metadata := cloneMetadata(init.Metadata)

		return &Provider{
			httpClient: httpClient,
			executor:   init.Executor,
			logger:     logger,
			config:     cfg,
			apiKey:     init.APIKey,
			model:      model,
			baseURL:    baseURL,
			metadata:   metadata,
		}, nil
	}
}

func (p *Provider) Name() string { return "gemini" }

func (p *Provider) Completion(ctx context.Context, req clientpkg.CompletionRequest) (clientpkg.CompletionResponse, error) {
	// Use generateContent endpoint.
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return clientpkg.CompletionResponse{}, errors.New("gemini: prompt is required")
	}

	payload := generateContentRequest{
		Model: pickModel(p.model, mergeMetadata(p.metadata, req.Metadata)),
		Contents: []content{
			{
				Role:  "user",
				Parts: []part{{Text: prompt}},
			},
		},
	}

	resp, err := p.performGenerateContent(ctx, payload)
	if err != nil {
		return clientpkg.CompletionResponse{}, err
	}

	text := resp.FirstText()
	if text == "" {
		return clientpkg.CompletionResponse{}, errors.New("gemini: empty response")
	}

	return clientpkg.CompletionResponse{Text: text, Raw: resp}, nil
}

func (p *Provider) Conversation(ctx context.Context) clientpkg.ConversationHandler {
	return &conversationHandler{provider: p, messages: make([]content, 0)}
}

func (p *Provider) Documents(ctx context.Context) clientpkg.DocumentHandler {
	return nil
}

type generateContentRequest struct {
	Model            string    `json:"model"`
	Contents         []content `json:"contents"`
	SafetySettings   any       `json:"safetySettings,omitempty"`
	GenerationConfig any       `json:"generationConfig,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text,omitempty"`
}

type generateContentResponse struct {
	Candidates []candidate `json:"candidates"`
}

type candidate struct {
	Content content `json:"content"`
}

func (r generateContentResponse) FirstText() string {
	for _, cand := range r.Candidates {
		for _, part := range cand.Content.Parts {
			if part.Text != "" {
				return part.Text
			}
		}
	}
	return ""
}

func (p *Provider) performGenerateContent(ctx context.Context, payload generateContentRequest) (generateContentResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return generateContentResponse{}, err
	}

	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", strings.TrimRight(p.baseURL, "/"), payload.Model, p.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return generateContentResponse{}, err
	}

	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return generateContentResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return generateContentResponse{}, fmt.Errorf("gemini: unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var out generateContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return generateContentResponse{}, err
	}

	p.logger.Info(ctx, "gemini: generate content", map[string]any{
		"model":   payload.Model,
		"latency": time.Since(start).String(),
	})

	return out, nil
}

func pickModel(defaultModel string, metadata map[string]any) string {
	if metadata == nil {
		return defaultModel
	}
	if value, ok := metadata["model"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return defaultModel
}

func mergeMetadata(base, override map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

func cloneMetadata(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

// conversationHandler maintains conversational history for Gemini.
type conversationHandler struct {
	provider *Provider
	messages []content
}

func (h *conversationHandler) Send(ctx context.Context, msg clientpkg.ConversationMessage) (clientpkg.ConversationReply, error) {
	userParts := []part{{Text: msg.Content}}
	h.messages = append(h.messages, content{Role: "user", Parts: userParts})

	payload := generateContentRequest{
		Model:    pickModel(h.provider.model, mergeMetadata(h.provider.metadata, msg.Metadata)),
		Contents: append([]content{}, h.messages...),
	}

	resp, err := h.provider.performGenerateContent(ctx, payload)
	if err != nil {
		return clientpkg.ConversationReply{}, err
	}

	text := resp.FirstText()
	if text == "" {
		return clientpkg.ConversationReply{}, errors.New("gemini: empty response")
	}

	replyContent := content{Role: "model", Parts: []part{{Text: text}}}
	h.messages = append(h.messages, replyContent)

	return clientpkg.ConversationReply{
		Message: clientpkg.ConversationMessage{Role: "model", Content: text},
		Raw:     resp,
	}, nil
}
