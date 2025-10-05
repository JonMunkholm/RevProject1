package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	clientpkg "github.com/JonMunkholm/RevProject1/internal/ai/client"
	"github.com/JonMunkholm/RevProject1/internal/ai/tool"
)

var (
	ErrMissingAPIKey     = errors.New("ai: openai api key not provided")
	ErrEmptyResponse     = errors.New("ai: openai returned no choices")
	ErrToolLoopExhausted = errors.New("ai: openai tool execution exceeded retries")
)

// Config captures optional provider settings such as base URL or model name.
type Config struct {
	BaseURL      string
	Model        string
	SystemPrompt string
	Logger       clientpkg.Logger
}

// Provider implements the client.Provider interface for OpenAI style APIs.
type Provider struct {
	name         string
	httpClient   *http.Client
	executor     *tool.Executor
	logger       clientpkg.Logger
	config       Config
	apiKey       string
	model        string
	baseURL      string
	metadata     map[string]any
	systemPrompt string
}

// Factory constructs an OpenAI provider compatible with the AI client.
func Factory(cfg Config) clientpkg.ProviderFactory {
	return func(init clientpkg.ProviderInit) (clientpkg.Provider, error) {
		if init.APIKey == "" {
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
			model = "gpt-4o-mini"
		}

		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = defaultBaseURL
		}

		metadata := sanitizeMetadata(cloneSchema(init.Metadata))

		return &Provider{
			name:         "openai",
			httpClient:   httpClient,
			executor:     init.Executor,
			logger:       logger,
			config:       cfg,
			apiKey:       init.APIKey,
			model:        model,
			baseURL:      baseURL,
			metadata:     metadata,
			systemPrompt: cfg.SystemPrompt,
		}, nil
	}
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Completion(ctx context.Context, req clientpkg.CompletionRequest) (clientpkg.CompletionResponse, error) {
	metadata := mergeMetadata(p.metadata, sanitizeMetadata(req.Metadata))
	messages := buildCompletionMessages(p.systemPrompt, metadata, req.Prompt)
	opts := requestOptionsFromMetadata(metadata)

	chatReq := chatCompletionRequest{
		Model:    pickModel(p.model, metadata),
		Messages: messages,
		Tools:    convertToolDescriptors(p.executor),
	}
	applyRequestOptions(&chatReq, opts)

	resp, err := p.performChat(ctx, chatReq)
	if err != nil {
		return clientpkg.CompletionResponse{}, err
	}

	choice, err := firstChoice(resp)
	if err != nil {
		return clientpkg.CompletionResponse{}, err
	}

	return clientpkg.CompletionResponse{
		Text: choice.Message.Content,
		Raw:  resp,
	}, nil
}

func (p *Provider) Conversation(context.Context) clientpkg.ConversationHandler {
	return &conversationHandler{provider: p, messages: make([]chatMessage, 0)}
}

func (p *Provider) Documents(context.Context) clientpkg.DocumentHandler {
	return nil
}

func (p *Provider) performChat(ctx context.Context, payload chatCompletionRequest) (chatCompletionResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return chatCompletionResponse{}, err
	}

	endpoint := p.baseURL + chatCompletionsPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return chatCompletionResponse{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	started := time.Now()
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return chatCompletionResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return chatCompletionResponse{}, fmt.Errorf("openai: unexpected status %d: %s", resp.StatusCode, string(data))
	}

	var out chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return chatCompletionResponse{}, err
	}

	p.logger.Info(ctx, "openai: chat completion", map[string]any{
		"model":   payload.Model,
		"latency": time.Since(started).String(),
	})

	return out, nil
}

func firstChoice(resp chatCompletionResponse) (chatCompletionChoice, error) {
	if len(resp.Choices) == 0 {
		return chatCompletionChoice{}, ErrEmptyResponse
	}
	return resp.Choices[0], nil
}

// requestOptions holds optional overrides supplied via metadata.
type requestOptions struct {
	Temperature      *float64
	MaxTokens        *int
	TopP             *float64
	PresencePenalty  *float64
	FrequencyPenalty *float64
	ToolChoice       interface{}
	ResponseFormat   interface{}
}

func requestOptionsFromMetadata(metadata map[string]any) requestOptions {
	return requestOptions{
		Temperature:      floatPointer(metadata["temperature"]),
		MaxTokens:        intPointer(metadata["max_tokens"]),
		TopP:             floatPointer(metadata["top_p"]),
		PresencePenalty:  floatPointer(metadata["presence_penalty"]),
		FrequencyPenalty: floatPointer(metadata["frequency_penalty"]),
		ToolChoice:       metadata["tool_choice"],
		ResponseFormat:   metadata["response_format"],
	}
}

func applyRequestOptions(req *chatCompletionRequest, opts requestOptions) {
	req.Temperature = opts.Temperature
	req.MaxTokens = opts.MaxTokens
	req.TopP = opts.TopP
	req.PresencePenalty = opts.PresencePenalty
	req.FrequencyPenalty = opts.FrequencyPenalty
	if opts.ToolChoice != nil {
		req.ToolChoice = opts.ToolChoice
	}
	if opts.ResponseFormat != nil {
		req.ResponseFormat = opts.ResponseFormat
	}
}

func pickModel(defaultModel string, metadata map[string]any) string {
	if metadata == nil {
		return defaultModel
	}
	if model, ok := metadata["model"].(string); ok && model != "" {
		return model
	}
	return defaultModel
}

func buildCompletionMessages(systemPrompt string, metadata map[string]any, prompt string) []chatMessage {
	messages := make([]chatMessage, 0, 3)
	if systemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
	}
	if addendum := extractSystemAddendum(metadata); addendum != "" {
		messages = append(messages, chatMessage{Role: "system", Content: addendum})
	}
	messages = append(messages, chatMessage{Role: "user", Content: prompt})
	return messages
}

func mergeMetadata(items ...map[string]any) map[string]any {
	merged := make(map[string]any)
	for _, item := range items {
		for k, v := range item {
			merged[k] = v
		}
	}
	return merged
}

func sanitizeMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		if k == "system" {
			continue
		}
		out[k] = v
	}
	return out
}

func extractSystemAddendum(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata["system_addendum"]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []string:
		return strings.Join(v, "\n")
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func floatPointer(value any) *float64 {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case float32:
		f := float64(v)
		return &f
	case float64:
		return &v
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return &f
		}
	case int:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return &f
		}
	}
	return nil
}

func intPointer(value any) *int {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case int:
		return &v
	case int32:
		i := int(v)
		return &i
	case int64:
		i := int(v)
		return &i
	case float64:
		i := int(v)
		return &i
	case json.Number:
		if i64, err := v.Int64(); err == nil {
			i := int(i64)
			return &i
		}
	case string:
		if i64, err := strconv.ParseInt(v, 10, 32); err == nil {
			i := int(i64)
			return &i
		}
	}
	return nil
}

type conversationHandler struct {
	provider *Provider
	messages []chatMessage
}

func (h *conversationHandler) Send(ctx context.Context, msg clientpkg.ConversationMessage) (clientpkg.ConversationReply, error) {
	metadata := mergeMetadata(h.provider.metadata, sanitizeMetadata(msg.Metadata))

	if len(h.messages) == 0 {
		if sp := h.provider.systemPrompt; sp != "" {
			h.messages = append(h.messages, chatMessage{Role: "system", Content: sp})
		}
		if addendum := extractSystemAddendum(metadata); addendum != "" {
			h.messages = append(h.messages, chatMessage{Role: "system", Content: addendum})
		}
	}

	role := msg.Role
	if role == "" {
		role = "user"
	}
	h.messages = append(h.messages, chatMessage{Role: role, Content: msg.Content})

	resp, assistant, err := h.provider.exchange(ctx, &h.messages, metadata)
	if err != nil {
		return clientpkg.ConversationReply{}, err
	}

	reply := clientpkg.ConversationReply{
		Message: clientpkg.ConversationMessage{
			Role:     assistant.Role,
			Content:  assistant.Content,
			Metadata: map[string]any{"finish_reason": resp.Choices[0].FinishReason, "usage": resp.Usage},
		},
		Raw: resp,
	}
	return reply, nil
}

func (p *Provider) exchange(ctx context.Context, history *[]chatMessage, metadata map[string]any) (chatCompletionResponse, chatMessage, error) {
	const maxToolIterations = 3
	var lastResp chatCompletionResponse
	var assistant chatMessage

	for i := 0; i < maxToolIterations; i++ {
		chatReq := chatCompletionRequest{
			Model:    pickModel(p.model, metadata),
			Messages: *history,
			Tools:    convertToolDescriptors(p.executor),
		}
		opts := requestOptionsFromMetadata(metadata)
		applyRequestOptions(&chatReq, opts)

		resp, err := p.performChat(ctx, chatReq)
		if err != nil {
			return chatCompletionResponse{}, chatMessage{}, err
		}
		lastResp = resp

		choice, err := firstChoice(resp)
		if err != nil {
			return chatCompletionResponse{}, chatMessage{}, err
		}

		assistant = choice.Message
		if assistant.Role == "" {
			assistant.Role = "assistant"
		}
		*history = append(*history, assistant)

		if len(assistant.ToolCalls) == 0 {
			return resp, assistant, nil
		}

		if err := p.handleToolCalls(ctx, history, assistant.ToolCalls); err != nil {
			return resp, assistant, err
		}
	}

	return lastResp, chatMessage{}, ErrToolLoopExhausted
}

func (p *Provider) handleToolCalls(ctx context.Context, history *[]chatMessage, calls []toolCall) error {
	if len(calls) == 0 || p.executor == nil {
		return nil
	}

	for _, call := range calls {
		if call.Function.Name == "" {
			continue
		}

		invocation := tool.Invocation{Name: call.Function.Name, Input: make(map[string]any)}
		if args := call.Function.Arguments; args != "" {
			var input map[string]any
			if err := json.Unmarshal([]byte(args), &input); err == nil {
				invocation.Input = input
			}
		}

		result, err := p.executor.Execute(ctx, invocation)
		var payload []byte
		if err != nil {
			payload, _ = json.Marshal(map[string]any{"error": err.Error()})
		} else {
			payload, _ = json.Marshal(result.Output)
		}

		toolMsg := chatMessage{
			Role:       "tool",
			ToolCallID: call.ID,
			Content:    string(payload),
		}
		*history = append(*history, toolMsg)
	}

	return nil
}
