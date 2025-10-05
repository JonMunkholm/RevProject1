package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/JonMunkholm/RevProject1/internal/ai/tool/audit"
	"github.com/JonMunkholm/RevProject1/internal/database"
)

// Store implements tool.InvocationStore backed by SQLC queries.
type Store struct {
	queries *database.Queries
}

func New(q *database.Queries) *Store { return &Store{queries: q} }

func (s *Store) InsertToolInvocation(ctx context.Context, params audit.InvocationRecord) error {
	var userID uuid.NullUUID
	if params.UserID != nil {
		if id, err := uuid.Parse(*params.UserID); err == nil {
			userID = uuid.NullUUID{UUID: id, Valid: true}
		}
	}

	var request pqtype.NullRawMessage
	if params.Request != nil {
		requestBytes, err := json.Marshal(params.Request)
		if err != nil {
			return err
		}
		if len(requestBytes) > 0 {
			request = pqtype.NullRawMessage{RawMessage: requestBytes, Valid: true}
		}
	}

	var response pqtype.NullRawMessage
	if params.Response != nil {
		responseBytes, err := json.Marshal(params.Response)
		if err != nil {
			return err
		}
		if len(responseBytes) > 0 {
			response = pqtype.NullRawMessage{RawMessage: responseBytes, Valid: true}
		}
	}

	var errorMessage sql.NullString
	if params.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *params.ErrorMessage, Valid: true}
	}

	status := params.Status
	if status == "" {
		status = "success"
	}

	return s.queries.InsertAIToolInvocation(ctx, database.InsertAIToolInvocationParams{
		UserID:       userID,
		ProviderID:   params.ProviderID,
		ToolName:     params.ToolName,
		Column4:      status,
		Request:      request,
		Response:     response,
		ErrorMessage: errorMessage,
	})
}
