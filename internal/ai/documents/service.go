package documents

import (
	"context"
	"time"

	"github.com/google/uuid"

	clientpkg "github.com/JonMunkholm/RevProject1/internal/ai/client"
)

// Store describes the persistence requirements for document jobs.
type Store interface {
	InsertJob(ctx context.Context, params CreateJobParams) (Job, error)
	UpdateJobStatus(ctx context.Context, companyID, jobID uuid.UUID, status string, errorMessage *string) error
	UpdateJobResponse(ctx context.Context, companyID, jobID uuid.UUID, response map[string]any) error
	GetJob(ctx context.Context, companyID, jobID uuid.UUID) (Job, error)
	ListJobs(ctx context.Context, companyID uuid.UUID, limit, offset int32) ([]Job, error)
	DeleteJob(ctx context.Context, companyID, jobID uuid.UUID) error
	NextQueuedJob(ctx context.Context) (Job, error)
}

// Job represents a document analysis task.
type Job struct {
	ID           uuid.UUID
	CompanyID    uuid.UUID
	UserID       uuid.UUID
	ProviderID   string
	Status       string
	Request      map[string]any
	Response     map[string]any
	ErrorMessage *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  *time.Time
}

type CreateJobParams struct {
	CompanyID  uuid.UUID
	UserID     uuid.UUID
	ProviderID string
	Status     string
	Request    map[string]any
}

// Service coordinates document job persistence.
type Service struct {
	store  Store
	logger clientpkg.Logger
}

func New(store Store, logger clientpkg.Logger) *Service {
	if logger == nil {
		logger = clientpkg.NewNoopLogger()
	}
	return &Service{store: store, logger: logger}
}

// Enqueue adds a new job to the queue.
func (s *Service) Enqueue(ctx context.Context, params CreateJobParams) (Job, error) {
	if params.Status == "" {
		params.Status = "queued"
	}
	job, err := s.store.InsertJob(ctx, params)
	if err != nil {
		return Job{}, err
	}
	s.logger.Info(ctx, "ai: document job created", map[string]any{"job_id": job.ID, "company_id": job.CompanyID})
	return job, nil
}

// MarkStatus updates a job status without a final response.
func (s *Service) MarkStatus(ctx context.Context, companyID, jobID uuid.UUID, status string, errMessage *string) error {
	if err := s.store.UpdateJobStatus(ctx, companyID, jobID, status, errMessage); err != nil {
		return err
	}
	s.logger.Info(ctx, "ai: document job status update", map[string]any{"job_id": jobID, "status": status})
	return nil
}

// Complete records a completed job response and marks the job finished.
func (s *Service) Complete(ctx context.Context, companyID, jobID uuid.UUID, response map[string]any) error {
	if err := s.store.UpdateJobResponse(ctx, companyID, jobID, response); err != nil {
		return err
	}
	s.logger.Info(ctx, "ai: document job completed", map[string]any{"job_id": jobID})
	return nil
}

// Job retrieves a single job.
func (s *Service) Job(ctx context.Context, companyID, jobID uuid.UUID) (Job, error) {
	return s.store.GetJob(ctx, companyID, jobID)
}

// Jobs lists jobs for the specified company.
func (s *Service) Jobs(ctx context.Context, companyID uuid.UUID, limit, offset int32) ([]Job, error) {
	return s.store.ListJobs(ctx, companyID, limit, offset)
}

// NextQueuedJob retrieves the next job in the queued state across all companies.
func (s *Service) NextQueuedJob(ctx context.Context) (Job, error) {
	return s.store.NextQueuedJob(ctx)
}

// Remove deletes a job and its data.
func (s *Service) Remove(ctx context.Context, companyID, jobID uuid.UUID) error {
	if err := s.store.DeleteJob(ctx, companyID, jobID); err != nil {
		return err
	}
	s.logger.Info(ctx, "ai: document job deleted", map[string]any{"job_id": jobID, "company_id": companyID})
	return nil
}
