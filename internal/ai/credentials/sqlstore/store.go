package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/JonMunkholm/RevProject1/internal/ai/credentials/dbresolver"
	"github.com/JonMunkholm/RevProject1/internal/database"
)

// Store implements dbresolver.CredentialStore backed by sqlc queries.
type Store struct {
	queries *database.Queries
}

// New returns a Store that persists provider credentials with the supplied query set.
func New(q *database.Queries) *Store { return &Store{queries: q} }

// ResolveCredential returns the highest-priority credential for the provided scope.
func (s *Store) ResolveCredential(ctx context.Context, companyID uuid.UUID, userID uuid.NullUUID, providerID string) (dbresolver.Record, error) {
	rows, err := s.queries.ListAIProviderCredentialsForResolver(ctx, database.ListAIProviderCredentialsForResolverParams{
		CompanyID:  companyID,
		ProviderID: providerID,
		UserID:     toNullUUID(userID),
	})
	if err != nil {
		return dbresolver.Record{}, err
	}
	if len(rows) == 0 {
		return dbresolver.Record{}, sql.ErrNoRows
	}
	return mapRecord(rows[0])
}

// GetCredential loads a credential by its identifier.
func (s *Store) GetCredential(ctx context.Context, id uuid.UUID) (dbresolver.Record, error) {
	row, err := s.queries.GetAIProviderCredential(ctx, id)
	if err != nil {
		return dbresolver.Record{}, err
	}
	return mapRecord(row)
}

// TouchCredential updates bookkeeping timestamps for the credential.
func (s *Store) TouchCredential(ctx context.Context, id uuid.UUID) error {
	return s.queries.TouchAIProviderCredentialByID(ctx, id)
}

// UpsertCredential inserts or updates the credential payload and metadata.
func (s *Store) UpsertCredential(ctx context.Context, record dbresolver.Record) (dbresolver.Record, error) {
	metadata, err := encodeJSON(record.Metadata)
	if err != nil {
		return dbresolver.Record{}, err
	}

	if record.ID != uuid.Nil {
		row, err := s.queries.UpdateAIProviderCredential(ctx, database.UpdateAIProviderCredentialParams{
			CredentialCipher: record.CredentialCipher,
			CredentialHash:   record.CredentialHash,
			Metadata:         toNullRawMessage(metadata),
			Label:            toNullString(record.Label),
			IsDefault:        sql.NullBool{Bool: record.IsDefault, Valid: true},
			LastTestedAt:     toNullTime(record.LastTestedAt),
			ID:               record.ID,
		})
		if err != nil {
			return dbresolver.Record{}, err
		}
		return mapRecord(row)
	}

	row, err := s.queries.InsertAIProviderCredential(ctx, database.InsertAIProviderCredentialParams{
		CompanyID:        record.CompanyID,
		UserID:           record.UserID,
		ProviderID:       record.ProviderID,
		CredentialCipher: record.CredentialCipher,
		CredentialHash:   record.CredentialHash,
		Metadata:         metadata,
		Label:            toNullString(record.Label),
		IsDefault:        record.IsDefault,
		LastTestedAt:     toNullTime(record.LastTestedAt),
	})
	if err != nil {
		return dbresolver.Record{}, err
	}
	return mapRecord(row)
}

// ListCompanyCredentials returns paginated credentials for the company.
func (s *Store) ListCompanyCredentials(ctx context.Context, companyID uuid.UUID, limit, offset int32) ([]dbresolver.Record, error) {
	rows, err := s.queries.ListAIProviderCredentialsByCompany(ctx, database.ListAIProviderCredentialsByCompanyParams{
		CompanyID: companyID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}
	return mapRecords(rows)
}

// ListProviderCredentials returns credentials for a provider/scope.
func (s *Store) ListProviderCredentials(ctx context.Context, companyID uuid.UUID, providerID string, userID uuid.NullUUID) ([]dbresolver.Record, error) {
	rows, err := s.queries.ListAIProviderCredentialsByScope(ctx, database.ListAIProviderCredentialsByScopeParams{
		CompanyID:  companyID,
		ProviderID: providerID,
		UserID:     toNullUUID(userID),
	})
	if err != nil {
		return nil, err
	}
	return mapRecords(rows)
}

// DeleteCredential removes a credential by id.
func (s *Store) DeleteCredential(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteAIProviderCredentialByID(ctx, id)
}

// ClearDefault resets the default flag for the given scope.
func (s *Store) ClearDefault(ctx context.Context, companyID uuid.UUID, providerID string, userID uuid.NullUUID) error {
	return s.queries.ClearDefaultAIProviderCredentials(ctx, database.ClearDefaultAIProviderCredentialsParams{
		CompanyID:  companyID,
		ProviderID: providerID,
		UserID:     toNullUUID(userID),
	})
}

func mapRecord(row database.AiProviderCredential) (dbresolver.Record, error) {
	meta, err := decodeJSON(row.Metadata)
	if err != nil {
		return dbresolver.Record{}, err
	}

	var label *string
	if row.Label.Valid {
		value := row.Label.String
		label = &value
	}

	var lastTested *time.Time
	if row.LastTestedAt.Valid {
		t := row.LastTestedAt.Time
		lastTested = &t
	}

	var lastUsed *time.Time
	if row.LastUsedAt.Valid {
		t := row.LastUsedAt.Time
		lastUsed = &t
	}

	var rotated *time.Time
	if row.RotatedAt.Valid {
		t := row.RotatedAt.Time
		rotated = &t
	}

	fingerprint := ""
	if row.Fingerprint.Valid {
		fingerprint = row.Fingerprint.String
	}

	return dbresolver.Record{
		ID:               row.ID,
		CompanyID:        row.CompanyID,
		UserID:           row.UserID,
		ProviderID:       row.ProviderID,
		CredentialCipher: row.CredentialCipher,
		CredentialHash:   row.CredentialHash,
		Metadata:         meta,
		Label:            label,
		IsDefault:        row.IsDefault,
		LastTestedAt:     lastTested,
		Fingerprint:      fingerprint,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		LastUsedAt:       lastUsed,
		RotatedAt:        rotated,
	}, nil
}

func mapRecords(rows []database.AiProviderCredential) ([]dbresolver.Record, error) {
	out := make([]dbresolver.Record, 0, len(rows))
	for _, row := range rows {
		record, err := mapRecord(row)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, nil
}

func encodeJSON(data map[string]any) (json.RawMessage, error) {
	if data == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(bytes), nil
}

func decodeJSON(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func toNullRawMessage(raw json.RawMessage) pqtype.NullRawMessage {
	if len(raw) == 0 {
		return pqtype.NullRawMessage{Valid: false}
	}
	return pqtype.NullRawMessage{RawMessage: raw, Valid: true}
}

func toNullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *value, Valid: true}
}

func toNullTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: *value, Valid: true}
}

func toNullUUID(value uuid.NullUUID) uuid.NullUUID {
	if !value.Valid {
		return uuid.NullUUID{}
	}
	return value
}
