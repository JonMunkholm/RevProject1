package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"

	"github.com/JonMunkholm/RevProject1/internal/ai/documents"
	"github.com/JonMunkholm/RevProject1/internal/database"
)

// Store implements documents.Store using the generated SQLC queries.
type Store struct {
	queries *database.Queries
}

func New(q *database.Queries) *Store { return &Store{queries: q} }

func (s *Store) InsertJob(ctx context.Context, params documents.CreateJobParams) (documents.Job, error) {
	reqBytes, err := json.Marshal(params.Request)
	if err != nil {
		return documents.Job{}, err
	}

	row, err := s.queries.InsertAIDocumentJob(ctx, database.InsertAIDocumentJobParams{
		CompanyID:  params.CompanyID,
		UserID:     params.UserID,
		ProviderID: params.ProviderID,
		Column4:    params.Status,
		Request:    reqBytes,
	})
	if err != nil {
		return documents.Job{}, err
	}
	return mapJob(row)
}

func (s *Store) UpdateJobStatus(ctx context.Context, companyID, jobID uuid.UUID, status string, errorMessage *string) error {
	return s.queries.UpdateAIDocumentJobStatus(ctx, database.UpdateAIDocumentJobStatusParams{
		ID:           jobID,
		CompanyID:    companyID,
		Status:       status,
		ErrorMessage: toNullString(errorMessage),
	})
}

func (s *Store) UpdateJobResponse(ctx context.Context, companyID, jobID uuid.UUID, response map[string]any) error {
	var payload pqtype.NullRawMessage
	if response != nil {
		bytes, err := json.Marshal(response)
		if err != nil {
			return err
		}
		payload = pqtype.NullRawMessage{RawMessage: bytes, Valid: true}
	}
	return s.queries.UpdateAIDocumentJobResponse(ctx, database.UpdateAIDocumentJobResponseParams{
		ID:        jobID,
		CompanyID: companyID,
		Response:  payload,
	})
}

func (s *Store) GetJob(ctx context.Context, companyID, jobID uuid.UUID) (documents.Job, error) {
	row, err := s.queries.GetAIDocumentJob(ctx, database.GetAIDocumentJobParams{ID: jobID, CompanyID: companyID})
	if err != nil {
		return documents.Job{}, err
	}
	return mapJob(row)
}

func (s *Store) ListJobs(ctx context.Context, companyID uuid.UUID, limit, offset int32) ([]documents.Job, error) {
	rows, err := s.queries.ListAIDocumentJobsByCompany(ctx, database.ListAIDocumentJobsByCompanyParams{
		CompanyID: companyID,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}
	jobs := make([]documents.Job, 0, len(rows))
	for _, r := range rows {
		job, err := mapJob(r)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *Store) DeleteJob(ctx context.Context, companyID, jobID uuid.UUID) error {
	return s.queries.DeleteAIDocumentJob(ctx, database.DeleteAIDocumentJobParams{ID: jobID, CompanyID: companyID})
}

func (s *Store) NextQueuedJob(ctx context.Context) (documents.Job, error) {
	row, err := s.queries.GetNextQueuedAIDocumentJob(ctx)
	if err != nil {
		return documents.Job{}, err
	}
	return mapJob(row)
}

func mapJob(row database.AiDocumentJob) (documents.Job, error) {
	request := map[string]any{}
	if len(row.Request) > 0 {
		if err := json.Unmarshal(row.Request, &request); err != nil {
			return documents.Job{}, err
		}
	}

	var response map[string]any
	if row.Response.Valid {
		if err := json.Unmarshal(row.Response.RawMessage, &response); err != nil {
			return documents.Job{}, err
		}
	}

	var errMessage *string
	if row.ErrorMessage.Valid {
		msg := row.ErrorMessage.String
		errMessage = &msg
	}

	var completed *time.Time
	if row.CompletedAt.Valid {
		t := row.CompletedAt.Time
		completed = &t
	}

	return documents.Job{
		ID:           row.ID,
		CompanyID:    row.CompanyID,
		UserID:       row.UserID,
		ProviderID:   row.ProviderID,
		Status:       row.Status,
		Request:      request,
		Response:     response,
		ErrorMessage: errMessage,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
		CompletedAt:  completed,
	}, nil
}

func toNullString(value *string) sql.NullString {
	if value == nil || *value == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *value, Valid: true}
}
