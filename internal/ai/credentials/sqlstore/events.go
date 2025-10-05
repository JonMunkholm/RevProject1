package sqlstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JonMunkholm/RevProject1/internal/database"
)

// Event represents a persisted credential event record.
type Event struct {
	ID          uuid.UUID
	CompanyID   uuid.UUID
	UserID      uuid.NullUUID
	ActorUserID uuid.NullUUID
	ProviderID  string
	Action      string
	Metadata    map[string]any
	CreatedAt   time.Time
}

// EventStore wraps credential event persistence.
type EventStore struct {
	queries *database.Queries
}

func NewEventStore(q *database.Queries) *EventStore { return &EventStore{queries: q} }

func (s *EventStore) Insert(ctx context.Context, params database.InsertAIProviderCredentialEventParams) error {
	return s.queries.InsertAIProviderCredentialEvent(ctx, params)
}

func (s *EventStore) List(ctx context.Context, companyID uuid.UUID, providerID string, limit, offset int32) ([]database.AiProviderCredentialEvent, error) {
	return s.queries.ListAIProviderCredentialEvents(ctx, database.ListAIProviderCredentialEventsParams{
		CompanyID:  companyID,
		ProviderID: providerID,
		Limit:      limit,
		Offset:     offset,
	})
}
