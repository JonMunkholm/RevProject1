package handler

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/ai"
	catalog "github.com/JonMunkholm/RevProject1/internal/ai/provider/catalog"
	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/google/uuid"
)

var errStatusNotImplemented = errors.New("ai: provider status check not implemented")

type credentialEventStore interface {
	Insert(ctx context.Context, params database.InsertAIProviderCredentialEventParams) error
	List(ctx context.Context, companyID uuid.UUID, providerID string, action *string, scope *string, actor uuid.NullUUID, limit, offset int32) ([]database.AiProviderCredentialEvent, error)
}

// AI exposes HTTP handlers for conversation and document job workflows.
type AI struct {
	Conversations     *ai.ConversationService
	Documents         *ai.DocumentService
	DefaultProvider   string
	Client            *ai.Client
	Resolver          ai.CredentialResolver
	APIKey            string
	CredentialStore   ai.CredentialStore
	CredentialCipher  ai.CredentialCipher
	CredentialEvents  credentialEventStore
	CredentialMetrics ai.CredentialMetrics
	ProviderCatalog   []ai.ProviderCatalogEntry
	CatalogLoader     *catalog.Loader
}

// Request types

type conversationResponse struct {
	ID        string         `json:"id"`
	Title     string         `json:"title,omitempty"`
	Provider  string         `json:"provider"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type messageResponse struct {
	ID        string         `json:"id"`
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

type documentJobResponse struct {
	ID          string         `json:"id"`
	Provider    string         `json:"provider"`
	Status      string         `json:"status"`
	Request     map[string]any `json:"request"`
	Response    map[string]any `json:"response,omitempty"`
	Error       *string        `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
}

type createConversationRequest struct {
	Title    string         `json:"title,omitempty"`
	Provider string         `json:"provider,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type appendMessageRequest struct {
	Content  string         `json:"content"`
	Role     string         `json:"role,omitempty"`
	Provider string         `json:"provider,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type createDocumentJobRequest struct {
	Provider     string         `json:"provider,omitempty"`
	Documents    []string       `json:"documents"`
	Instructions string         `json:"instructions,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type upsertProviderCredentialRequest struct {
	Provider     string         `json:"provider,omitempty"`
	APIKey       string         `json:"apiKey"`
	Model        string         `json:"model,omitempty"`
	BaseURL      string         `json:"baseUrl,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Scope        string         `json:"scope,omitempty"`
	UserID       string         `json:"userId,omitempty"`
	Label        string         `json:"label,omitempty"`
	MakeDefault  bool           `json:"makeDefault,omitempty"`
	CredentialID string         `json:"credentialId,omitempty"`
}

type listResponse[T any] struct {
	Items      []T   `json:"items"`
	NextOffset int32 `json:"nextOffset"`
}

type providerCredentialResponse struct {
	ID          string         `json:"id"`
	ProviderID  string         `json:"provider"`
	Scope       string         `json:"scope"`
	UserID      *string        `json:"userId,omitempty"`
	Label       string         `json:"label,omitempty"`
	Fingerprint string         `json:"fingerprint"`
	IsDefault   bool           `json:"isDefault"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	LastUsedAt  *time.Time     `json:"lastUsedAt,omitempty"`
	RotatedAt   *time.Time     `json:"rotatedAt,omitempty"`
	KeySuffix   string         `json:"keySuffix,omitempty"`
}

type providerCredentialEventResponse struct {
	ID        string         `json:"id"`
	Action    string         `json:"action"`
	ActorID   *string        `json:"actorId,omitempty"`
	UserID    *string        `json:"userId,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

// Shared utility functions

func paginationParams(r *http.Request, fallback int32) (int32, int32) {
	limit := fallback
	offset := int32(0)

	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = int32(v)
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = int32(v)
		}
	}
	return limit, offset
}

func isHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return map[string]any{}
	}
	clone := make(map[string]any, len(metadata))
	for k, v := range metadata {
		clone[k] = v
	}
	return clone
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func scopeFromUserID(user uuid.NullUUID) string {
	if user.Valid {
		return "user"
	}
	return "company"
}

func nullUUID(id uuid.UUID) uuid.NullUUID {
	if id == uuid.Nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: id, Valid: true}
}
