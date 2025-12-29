package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/ai/documents"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/config"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

// CreateDocumentJob queues a document analysis request.
func (h *AI) CreateDocumentJob(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Documents == nil {
		RespondWithError(w, http.StatusInternalServerError, "document analysis unavailable", errors.New("document service not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	var req createDocumentJobRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		return
	}
	if len(req.Documents) == 0 {
		RespondWithError(w, http.StatusBadRequest, "documents are required", errors.New("missing documents"))
		return
	}

	providerID := req.Provider
	if providerID == "" {
		providerID = h.DefaultProvider
	}

	requestPayload := map[string]any{
		"documents": req.Documents,
	}
	if req.Instructions != "" {
		requestPayload["instructions"] = req.Instructions
	}
	if len(req.Metadata) > 0 {
		requestPayload["metadata"] = req.Metadata
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	job, err := h.Documents.Enqueue(ctx, documents.CreateJobParams{
		CompanyID:  session.CompanyID,
		UserID:     session.UserID,
		ProviderID: providerID,
		Request:    requestPayload,
	})
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to queue job", err)
		return
	}

	RespondWithJSON(w, http.StatusAccepted, jobToResponse(job))
}

// ListDocumentJobs lists queued and historical jobs for the company.
func (h *AI) ListDocumentJobs(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Documents == nil {
		RespondWithError(w, http.StatusInternalServerError, "document analysis unavailable", errors.New("document service not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	limit, offset := paginationParams(r, config.DefaultJobLimit)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	jobs, err := h.Documents.Jobs(ctx, session.CompanyID, limit, offset)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to list jobs", err)
		return
	}

	resp := listResponse[documentJobResponse]{NextOffset: offset + limit}
	for _, job := range jobs {
		resp.Items = append(resp.Items, jobToResponse(job))
	}

	RespondWithJSON(w, http.StatusOK, resp)
}

// GetDocumentJob returns a single job by ID.
func (h *AI) GetDocumentJob(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Documents == nil {
		RespondWithError(w, http.StatusInternalServerError, "document analysis unavailable", errors.New("document service not configured"))
		return
	}

	session, ok := auth.SessionFromContext(r.Context())
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid job id", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	job, err := h.Documents.Job(ctx, session.CompanyID, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			RespondWithError(w, http.StatusNotFound, "job not found", err)
			return
		}
		RespondWithError(w, http.StatusInternalServerError, "failed to load job", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, jobToResponse(job))
}

func jobToResponse(job documents.Job) documentJobResponse {
	return documentJobResponse{
		ID:          job.ID.String(),
		Provider:    job.ProviderID,
		Status:      job.Status,
		Request:     job.Request,
		Response:    job.Response,
		Error:       job.ErrorMessage,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
		CompletedAt: job.CompletedAt,
	}
}
