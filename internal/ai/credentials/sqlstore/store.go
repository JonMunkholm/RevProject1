package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/JonMunkholm/RevProject1/internal/ai/credentials/dbresolver"
	"github.com/JonMunkholm/RevProject1/internal/database"
)

// Store implements dbresolver.CredentialStore backed by sqlc queries.
type Store struct {
	queries *database.Queries
}

// New returns a Store that persists provider credentials with the supplied query set.
func New(q *database.Queries) *Store { return &Store{queries: q} }

// FetchCredential loads an encrypted credential record from the database.
func (s *Store) FetchCredential(ctx context.Context, companyID, userID uuid.UUID, providerID string) (dbresolver.Record, error) {
	row, err := s.queries.GetAIProviderCredential(ctx, database.GetAIProviderCredentialParams{
		CompanyID:  companyID,
		UserID:     userID,
		ProviderID: providerID,
	})
	if err != nil {
		return dbresolver.Record{}, err
	}

	meta, err := decodeJSON(row.Metadata)
	if err != nil {
		return dbresolver.Record{}, err
	}

	return dbresolver.Record{
		CompanyID:        row.CompanyID,
		UserID:           row.UserID,
		ProviderID:       row.ProviderID,
		CredentialCipher: row.CredentialCipher,
		CredentialHash:   row.CredentialHash,
		Metadata:         meta,
	}, nil
}

// TouchCredential updates the bookkeeping timestamps for the credential.
func (s *Store) TouchCredential(ctx context.Context, companyID, userID uuid.UUID, providerID string) error {
	return s.queries.TouchAIProviderCredential(ctx, database.TouchAIProviderCredentialParams{
		CompanyID:  companyID,
		UserID:     userID,
		ProviderID: providerID,
	})
}

// UpsertCredential inserts or updates the credential payload and metadata.
func (s *Store) UpsertCredential(ctx context.Context, record dbresolver.Record) error {
	metadata, err := encodeJSON(record.Metadata)
	if err != nil {
		return err
	}

	_, err = s.queries.UpdateAIProviderCredential(ctx, database.UpdateAIProviderCredentialParams{
		CompanyID:        record.CompanyID,
		UserID:           record.UserID,
		CredentialCipher: record.CredentialCipher,
		CredentialHash:   record.CredentialHash,
		Metadata:         metadata,
		ProviderID:       record.ProviderID,
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = s.queries.InsertAIProviderCredential(ctx, database.InsertAIProviderCredentialParams{
		CompanyID:        record.CompanyID,
		UserID:           record.UserID,
		ProviderID:       record.ProviderID,
		CredentialCipher: record.CredentialCipher,
		CredentialHash:   record.CredentialHash,
		Column6:          metadata,
	})
	return err
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
