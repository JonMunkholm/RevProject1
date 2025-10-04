package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/JonMunkholm/RevProject1/internal/ai/credentials"
	"github.com/JonMunkholm/RevProject1/internal/ai/tool"
)

// Provider represents an abstract large language model implementation.
type Provider interface {
	Name() string
	Completion(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	Conversation(ctx context.Context) ConversationHandler
	Documents(ctx context.Context) DocumentHandler
}

// ProviderFactory builds a Provider instance using the supplied options.
type ProviderFactory func(ProviderInit) (Provider, error)

// ProviderInit carries runtime configuration for a provider instance.
type ProviderInit struct {
	APIKey     string
	HTTPClient *http.Client
	Metadata   map[string]any
	Executor   *tool.Executor
}

// Config configures the AI client.
type Config struct {
	Providers       map[string]ProviderFactory
	DefaultProvider string
	HTTPClient      *http.Client
	Logger          Logger
	Tools           []tool.Tool
	Credentials     credentials.Resolver
}

// UserOptions describe the provider preferences for a specific request or user.
type UserOptions struct {
	Provider  string
	APIKey    string
	APIKeyRef string
	Metadata  map[string]any
}

// CompletionRequest encapsulates a text generation request.
type CompletionRequest struct {
	Prompt   string
	Metadata map[string]any
}

// CompletionResponse is the portable form of a provider response.
type CompletionResponse struct {
	Text string
	Raw  any
}

// ConversationMessage represents a single message in a conversational flow.
type ConversationMessage struct {
	Role     string
	Content  string
	Metadata map[string]any
}

// ConversationReply is the provider response for a conversation exchange.
type ConversationReply struct {
	Message ConversationMessage
	Raw     any
}

// ConversationHandler handles conversational exchanges.
type ConversationHandler interface {
	Send(ctx context.Context, message ConversationMessage) (ConversationReply, error)
}

// DocumentRequest represents a request to perform analysis on one or more documents.
type DocumentRequest struct {
	Documents []string
	Metadata  map[string]any
}

// DocumentResponse represents the provider output for a document analysis request.
type DocumentResponse struct {
	Summary string
	Raw     any
}

// DocumentHandler handles document analysis operations.
type DocumentHandler interface {
	Analyze(ctx context.Context, request DocumentRequest) (DocumentResponse, error)
}

// Logger instruments AI client operations.
type Logger interface {
	Info(ctx context.Context, msg string, attrs ...any)
	Error(ctx context.Context, msg string, err error, attrs ...any)
}

type noopLogger struct{}

func (noopLogger) Info(context.Context, string, ...any)         {}
func (noopLogger) Error(context.Context, string, error, ...any) {}

// NewNoopLogger returns a logger that performs no operations.
func NewNoopLogger() Logger { return noopLogger{} }

var (
	ErrProviderNotConfigured    = errors.New("ai: provider not configured")
	ErrCapabilityNotImplemented = errors.New("ai: capability not implemented")
)

// Client orchestrates access to different AI providers.
type Client struct {
	httpClient      *http.Client
	defaultProvider string

	mu        sync.RWMutex
	factories map[string]ProviderFactory
	cache     map[string]Provider

	logger Logger

	tools *tool.Registry
	exec  *tool.Executor
	creds credentials.Resolver
}

// NewClient builds a client from the supplied configuration.
func NewClient(cfg Config) (*Client, error) {
	if len(cfg.Providers) == 0 {
		return nil, errors.New("ai: at least one provider factory must be configured")
	}

	defaultProvider := cfg.DefaultProvider
	if defaultProvider == "" {
		for name := range cfg.Providers {
			defaultProvider = name
			break
		}
	}
	if _, ok := cfg.Providers[defaultProvider]; !ok {
		return nil, fmt.Errorf("ai: default provider %q not registered", defaultProvider)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	logger := cfg.Logger
	if logger == nil {
		logger = noopLogger{}
	}

	registry := tool.NewRegistry()
	for _, t := range cfg.Tools {
		registry.Register(t)
	}

	creds := cfg.Credentials
	if creds == nil {
		creds = credentials.NewNoopResolver()
	}

	c := &Client{
		httpClient:      client,
		defaultProvider: defaultProvider,
		factories:       cfg.Providers,
		cache:           make(map[string]Provider),
		logger:          logger,
		tools:           registry,
		exec:            tool.NewExecutor(registry, logger),
		creds:           creds,
	}

	return c, nil
}

// RegisterProvider allows adding a new provider at runtime.
func (c *Client) RegisterProvider(id string, factory ProviderFactory) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.factories == nil {
		c.factories = make(map[string]ProviderFactory)
	}
	c.factories[id] = factory

	prefix := id + "::"
	for key := range c.cache {
		if strings.HasPrefix(key, prefix) {
			delete(c.cache, key)
		}
	}

	c.logger.Info(context.Background(), "ai: provider registered", "provider", id)
}

// Completion dispatches the request to the appropriate provider based on the supplied user options.
func (c *Client) Completion(ctx context.Context, opts UserOptions, req CompletionRequest) (CompletionResponse, error) {
	provider, err := c.providerFor(ctx, opts)
	if err != nil {
		return CompletionResponse{}, err
	}
	return provider.Completion(ctx, req)
}

// Conversation returns a conversation handler for the chosen provider.
func (c *Client) Conversation(ctx context.Context, opts UserOptions) ConversationHandler {
	provider, err := c.providerFor(ctx, opts)
	if err != nil {
		return noopConversationHandler{}
	}
	if handler := provider.Conversation(ctx); handler != nil {
		return handler
	}
	return noopConversationHandler{}
}

// Documents returns a document analysis handler for the chosen provider.
func (c *Client) Documents(ctx context.Context, opts UserOptions) DocumentHandler {
	provider, err := c.providerFor(ctx, opts)
	if err != nil {
		return noopDocumentHandler{}
	}
	if handler := provider.Documents(ctx); handler != nil {
		return handler
	}
	return noopDocumentHandler{}
}

// RegisterTool adds a new tool implementation to the registry.
func (c *Client) RegisterTool(t tool.Tool) {
	if t == nil {
		return
	}
	c.tools.Register(t)
	c.exec = tool.NewExecutor(c.tools, c.logger)
	c.logger.Info(context.Background(), "ai: tool registered", "tool", t.Name())
}

func (c *Client) Tool(name string) (tool.Tool, bool) {
	return c.tools.Get(name)
}

func (c *Client) Tools() []tool.Tool {
	return c.tools.List()
}

func (c *Client) ToolDescriptors() []tool.Descriptor {
	return c.tools.Descriptors()
}

func (c *Client) ToolExecutor() *tool.Executor {
	return c.exec
}

func (c *Client) providerFor(ctx context.Context, opts UserOptions) (Provider, error) {
	c.mu.RLock()
	factories := c.factories
	defaultProvider := c.defaultProvider
	c.mu.RUnlock()

	if len(factories) == 0 {
		c.logger.Error(ctx, "ai: no providers configured", ErrProviderNotConfigured)
		return nil, ErrProviderNotConfigured
	}

	providerID := opts.Provider
	if providerID == "" {
		providerID = defaultProvider
	}

	factory, ok := factories[providerID]
	if !ok {
		err := fmt.Errorf("%w: %s", ErrProviderNotConfigured, providerID)
		c.logger.Error(ctx, "ai: provider not configured", err, "provider", providerID)
		return nil, err
	}

	apiKey := opts.APIKey
	if apiKey == "" && opts.APIKeyRef != "" {
		resolved, err := c.creds.Resolve(ctx, opts.APIKeyRef)
		if err != nil {
			c.logger.Error(ctx, "ai: credential resolve failed", err, "provider", providerID)
			return nil, err
		}
		apiKey = resolved
		c.creds.Audit(ctx, opts.APIKeyRef, map[string]any{"provider": providerID})
	}

	cacheKey := providerCacheKey(providerID, apiKey, opts.APIKeyRef)

	c.mu.RLock()
	provider, cached := c.cache[cacheKey]
	c.mu.RUnlock()

	if cached {
		c.logger.Info(ctx, "ai: provider cache hit", "provider", providerID)
		return provider, nil
	}

	init := ProviderInit{
		APIKey:     apiKey,
		HTTPClient: c.httpClient,
		Metadata:   opts.Metadata,
		Executor:   c.exec,
	}

	instance, err := factory(init)
	if err != nil {
		c.logger.Error(ctx, "ai: provider init failed", err, "provider", providerID)
		return nil, err
	}

	c.mu.Lock()
	c.cache[cacheKey] = instance
	c.mu.Unlock()

	c.logger.Info(ctx, "ai: provider initialised", "provider", providerID)
	return instance, nil
}

func providerCacheKey(providerID, apiKey, apiKeyRef string) string {
	builder := strings.Builder{}
	builder.WriteString(providerID)
	builder.WriteString("::")
	if apiKey != "" {
		builder.WriteString(hashString(apiKey))
	} else if apiKeyRef != "" {
		builder.WriteString("ref:")
		builder.WriteString(hashString(apiKeyRef))
	} else {
		builder.WriteString("anon")
	}
	return builder.String()
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

type noopConversationHandler struct{}

type noopDocumentHandler struct{}

func (noopConversationHandler) Send(context.Context, ConversationMessage) (ConversationReply, error) {
	return ConversationReply{}, ErrCapabilityNotImplemented
}

func (noopDocumentHandler) Analyze(context.Context, DocumentRequest) (DocumentResponse, error) {
	return DocumentResponse{}, ErrCapabilityNotImplemented
}
