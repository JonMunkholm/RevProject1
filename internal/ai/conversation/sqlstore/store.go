package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/JonMunkholm/RevProject1/internal/ai/conversation"
	"github.com/JonMunkholm/RevProject1/internal/database"
)

// Store implements conversation.Store backed by the generated SQLC queries.
type Store struct {
	queries *database.Queries
}

func New(q *database.Queries) *Store { return &Store{queries: q} }

func (s *Store) CreateSession(ctx context.Context, params conversation.CreateSessionParams) (conversation.Session, error) {
	meta, err := encodeJSON(params.Metadata)
	if err != nil {
		return conversation.Session{}, err
	}

	row, err := s.queries.CreateAIConversationSession(ctx, database.CreateAIConversationSessionParams{
		CompanyID:  params.CompanyID,
		UserID:     params.UserID,
		ProviderID: params.ProviderID,
		Title:      nullString(params.Title),
		Column5:    meta,
	})
	if err != nil {
		return conversation.Session{}, err
	}
	return mapSession(row)
}

func (s *Store) UpdateSessionTitle(ctx context.Context, id, companyID uuid.UUID, title string) error {
	return s.queries.UpdateAIConversationSessionTitle(ctx, database.UpdateAIConversationSessionTitleParams{
		ID:        id,
		CompanyID: companyID,
		Title:     nullString(title),
	})
}

func (s *Store) ListSessions(ctx context.Context, companyID uuid.UUID, limit, offset int32) ([]conversation.Session, error) {
	rows, err := s.queries.ListAIConversationSessionsByCompany(ctx, database.ListAIConversationSessionsByCompanyParams{
		CompanyID: companyID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}

	sessions := make([]conversation.Session, 0, len(rows))
	for _, r := range rows {
		session, err := mapSession(r)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s *Store) DeleteSession(ctx context.Context, id, companyID uuid.UUID) error {
	return s.queries.DeleteAIConversationSession(ctx, database.DeleteAIConversationSessionParams{ID: id, CompanyID: companyID})
}

func (s *Store) InsertMessage(ctx context.Context, params conversation.CreateMessageParams) (conversation.Message, error) {
	meta, err := encodeJSON(params.Metadata)
	if err != nil {
		return conversation.Message{}, err
	}

	row, err := s.queries.InsertAIConversationMessage(ctx, database.InsertAIConversationMessageParams{
		SessionID: params.SessionID,
		Role:      params.Role,
		Content:   params.Content,
		Column4:   meta,
	})
	if err != nil {
		return conversation.Message{}, err
	}
	return mapMessage(row)
}

func (s *Store) ListMessages(ctx context.Context, sessionID uuid.UUID) ([]conversation.Message, error) {
	rows, err := s.queries.ListAIConversationMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	msgs := make([]conversation.Message, 0, len(rows))
	for _, r := range rows {
		msg, err := mapMessage(r)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

func (s *Store) DeleteMessages(ctx context.Context, sessionID uuid.UUID) error {
	return s.queries.DeleteAIConversationMessagesForSession(ctx, sessionID)
}

func mapSession(row database.AiConversationSession) (conversation.Session, error) {
	meta, err := decodeJSON(row.Metadata)
	if err != nil {
		return conversation.Session{}, err
	}
	title := ""
	if row.Title.Valid {
		title = row.Title.String
	}
	return conversation.Session{
		ID:         row.ID,
		CompanyID:  row.CompanyID,
		UserID:     row.UserID,
		ProviderID: row.ProviderID,
		Title:      title,
		Metadata:   meta,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}, nil
}

func mapMessage(row database.AiConversationMessage) (conversation.Message, error) {
	meta, err := decodeJSON(row.Metadata)
	if err != nil {
		return conversation.Message{}, err
	}
	return conversation.Message{
		ID:        row.ID,
		SessionID: row.SessionID,
		Role:      row.Role,
		Content:   row.Content,
		Metadata:  meta,
		CreatedAt: row.CreatedAt,
	}, nil
}

func encodeJSON(data map[string]any) (any, error) {
	if data == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return bytes, nil
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

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: value, Valid: true}
}
