package ai

import (
	c "github.com/JonMunkholm/RevProject1/internal/ai/client"
	"github.com/JonMunkholm/RevProject1/internal/ai/conversation"
	cred "github.com/JonMunkholm/RevProject1/internal/ai/credentials"
	"github.com/JonMunkholm/RevProject1/internal/ai/credentials/dbresolver"
	doc "github.com/JonMunkholm/RevProject1/internal/ai/documents"
	"github.com/JonMunkholm/RevProject1/internal/ai/provider/openai"
	t "github.com/JonMunkholm/RevProject1/internal/ai/tool"
	"github.com/JonMunkholm/RevProject1/internal/ai/tool/audit"
)

type (
	Config              = c.Config
	Client              = c.Client
	UserOptions         = c.UserOptions
	CompletionRequest   = c.CompletionRequest
	CompletionResponse  = c.CompletionResponse
	ConversationMessage = c.ConversationMessage
	ConversationReply   = c.ConversationReply
	DocumentRequest     = c.DocumentRequest
	DocumentResponse    = c.DocumentResponse
	Provider            = c.Provider
	ProviderFactory     = c.ProviderFactory
	ProviderInit        = c.ProviderInit
	Logger              = c.Logger

	Tool           = t.Tool
	ToolHandler    = t.Handler
	ToolRegistry   = t.Registry
	ToolDescriptor = t.Descriptor
	ToolInvocation = t.Invocation
	ToolResult     = t.Result
	ToolExecutor   = t.Executor

	CredentialResolver = cred.Resolver
	CredentialLogger   = cred.Logger

	ConversationService       = conversation.Service
	ConversationSession       = conversation.Session
	ConversationMessageRecord = conversation.Message
	DocumentService           = doc.Service
	DocumentJob               = doc.Job
	ToolAuditor               = audit.AuditingExecutor
	CredentialRecord          = dbresolver.Record
	CredentialCipher          = dbresolver.Cipher
	CredentialStore           = dbresolver.CredentialStore
	CredentialReference       = dbresolver.Reference
)

var (
	ErrProviderNotConfigured    = c.ErrProviderNotConfigured
	ErrCapabilityNotImplemented = c.ErrCapabilityNotImplemented
)

func NewClient(cfg Config) (*Client, error) { return c.NewClient(cfg) }

func NewToolRegistry() *ToolRegistry { return t.NewRegistry() }

func NewToolExecutor(r *ToolRegistry, logger Logger) *ToolExecutor { return t.NewExecutor(r, logger) }

func NewAuditingExecutor(inner *ToolExecutor, store audit.InvocationStore) *ToolAuditor {
	return audit.NewAuditingExecutor(inner, store)
}

func NewConversationService(store conversation.Store, logger Logger) *ConversationService {
	return conversation.New(store, logger)
}

func NewDocumentService(store doc.Store, logger Logger) *DocumentService {
	return doc.New(store, logger)
}

func NewNoopCredentialResolver() CredentialResolver { return cred.NewNoopResolver() }

func NewNoopCredentialLogger() CredentialLogger { return cred.NewNoopLogger() }

func NewDBCredentialResolver(store CredentialStore, cipher CredentialCipher, logger CredentialLogger) CredentialResolver {
	return dbresolver.New(store, cipher, logger)
}

func ParseCredentialReference(ref string) (CredentialReference, error) {
	return dbresolver.ParseReference(ref)
}

func NewOpenAIProviderFactory(cfg openai.Config) ProviderFactory { return openai.Factory(cfg) }
