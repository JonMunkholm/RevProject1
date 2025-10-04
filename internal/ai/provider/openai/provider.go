package openai

import (
	"context"
	"errors"
	"net/http"

	clientpkg "github.com/JonMunkholm/RevProject1/internal/ai/client"
	"github.com/JonMunkholm/RevProject1/internal/ai/tool"
)

var (
	ErrNotImplemented = errors.New("ai: provider not yet implemented")
)

// Config captures optional provider settings such as base URL or model name.
type Config struct {
	BaseURL string
	Model   string
}

// Provider implements the client.Provider interface for OpenAI style APIs.
type Provider struct {
	name       string
	httpClient *http.Client
	executor   *tool.Executor
	logger     clientpkg.Logger
	config     Config
}

// Factory constructs an OpenAI provider compatible with the AI client.
func Factory(cfg Config) clientpkg.ProviderFactory {
	return func(init clientpkg.ProviderInit) (clientpkg.Provider, error) {
		httpClient := init.HTTPClient
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
		logger := clientpkg.NewNoopLogger()
		return &Provider{
			name:       "openai",
			httpClient: httpClient,
			executor:   init.Executor,
			logger:     logger,
			config:     cfg,
		}, nil
	}
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Completion(context.Context, clientpkg.CompletionRequest) (clientpkg.CompletionResponse, error) {
	return clientpkg.CompletionResponse{}, ErrNotImplemented
}

func (p *Provider) Conversation(context.Context) clientpkg.ConversationHandler {
	return nil
}

func (p *Provider) Documents(context.Context) clientpkg.DocumentHandler {
	return nil
}
