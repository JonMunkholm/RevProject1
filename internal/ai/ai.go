package ai

import (
	c "github.com/JonMunkholm/RevProject1/internal/ai/client"
	"github.com/JonMunkholm/RevProject1/internal/ai/conversation"
	conversationsqlstore "github.com/JonMunkholm/RevProject1/internal/ai/conversation/sqlstore"
	cred "github.com/JonMunkholm/RevProject1/internal/ai/credentials"
	"github.com/JonMunkholm/RevProject1/internal/ai/credentials/aescipher"
	"github.com/JonMunkholm/RevProject1/internal/ai/credentials/dbresolver"
	credentialsqlstore "github.com/JonMunkholm/RevProject1/internal/ai/credentials/sqlstore"
	doc "github.com/JonMunkholm/RevProject1/internal/ai/documents"
	documentsqlstore "github.com/JonMunkholm/RevProject1/internal/ai/documents/sqlstore"
	"github.com/JonMunkholm/RevProject1/internal/ai/provider/catalog"
	"github.com/JonMunkholm/RevProject1/internal/ai/provider/openai"
	t "github.com/JonMunkholm/RevProject1/internal/ai/tool"
	"github.com/JonMunkholm/RevProject1/internal/ai/tool/audit"
	toolsqlstore "github.com/JonMunkholm/RevProject1/internal/ai/tool/sqlstore"
	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/JonMunkholm/RevProject1/internal/ai/metrics"
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

	Tool                = t.Tool
	ToolHandler         = t.Handler
	ToolRegistry        = t.Registry
	ToolDescriptor      = t.Descriptor
	ToolInvocation      = t.Invocation
	ToolResult          = t.Result
	ToolExecutor        = t.Executor
	ToolInvocationStore = audit.InvocationStore

	CredentialResolver   = cred.Resolver
	CredentialLogger     = cred.Logger
	ProviderCatalogEntry = catalog.Entry
	ProviderField        = catalog.Field

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
	CredentialEventStore      = credentialsqlstore.EventStore
	CredentialMetrics         = metrics.CredentialMetrics
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

func NewNoopLogger() Logger { return c.NewNoopLogger() }

func ParseCredentialReference(ref string) (CredentialReference, error) {
	return dbresolver.ParseReference(ref)
}

func NewOpenAIProviderFactory(cfg openai.Config) ProviderFactory { return openai.Factory(cfg) }

func NewAESCipher(key []byte) (CredentialCipher, error) { return aescipher.New(key) }

func NewAESCipherFromBase64(encoded string) (CredentialCipher, error) {
	return aescipher.NewFromBase64(encoded)
}

func NewCredentialSQLStore(q *database.Queries) CredentialStore {
	return credentialsqlstore.New(q)
}

func NewCredentialEventSQLStore(q *database.Queries) *CredentialEventStore {
	return credentialsqlstore.NewEventStore(q)
}

func NewCredentialMetrics(reg prometheus.Registerer) CredentialMetrics {
	return metrics.NewCredentialMetrics(reg)
}

func NewConversationSQLStore(q *database.Queries) conversation.Store {
	return conversationsqlstore.New(q)
}

func NewDocumentSQLStore(q *database.Queries) doc.Store {
	return documentsqlstore.New(q)
}

func NewToolAuditSQLStore(q *database.Queries) audit.InvocationStore {
	return toolsqlstore.New(q)
}

func ProviderCatalog() []ProviderCatalogEntry { return catalog.Catalog() }

func ProviderCatalogEntryByID(id string) (ProviderCatalogEntry, bool) {
	return catalog.Lookup(id)
}

// WithSystemAddendum appends a handler-specific system addendum to the metadata map.
func WithSystemAddendum(metadata map[string]any, addendum string) map[string]any {
	if addendum == "" {
		return metadata
	}
	if metadata == nil {
		metadata = make(map[string]any)
	}
	switch existing := metadata["system_addendum"].(type) {
	case string:
		metadata["system_addendum"] = []string{existing, addendum}
	case []string:
		metadata["system_addendum"] = append(existing, addendum)
	case []any:
		existing = append(existing, addendum)
		metadata["system_addendum"] = existing
	default:
		metadata["system_addendum"] = addendum
	}
	return metadata
}
