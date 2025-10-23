package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/JonMunkholm/RevProject1/internal/contextutil"
	"github.com/JonMunkholm/RevProject1/internal/retrieval"
	"github.com/JonMunkholm/RevProject1/internal/stage1"
)

type server struct {
	retrieval         *retrieval.Service
	db                *sql.DB
	guardrailsEnabled bool
}

func main() {
	log.SetFlags(0)
	if err := run(); err != nil {
		log.Fatalf("api: %v", err)
	}
}

func run() error {
	_ = godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return errors.New("DB_URL must be set")
	}
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		return errors.New("OPENAI_API_KEY must be set")
	}
	openAIBase := os.Getenv("OPENAI_API_BASE")
	openAIProject := os.Getenv("OPENAI_PROJECT_ID")
	port := os.Getenv("PORT")
	if strings.TrimSpace(port) == "" {
		port = ":8080"
	}
	guardrailsEnabled := parseBoolEnv("GUARDRAILS_ENABLED", false)

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(4)
	db.SetMaxOpenConns(8)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	if err := stage1.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}

	retrievalSvc, err := retrieval.NewService(retrieval.Config{
		DB:                db,
		OpenAIKey:         openAIKey,
		OpenAIURL:         openAIBase,
		ProjectID:         openAIProject,
		GuardrailsEnabled: guardrailsEnabled,
	})
	if err != nil {
		return fmt.Errorf("init retrieval: %w", err)
	}

	srv := &server{retrieval: retrievalSvc, db: db, guardrailsEnabled: guardrailsEnabled}
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/search", http.StatusSeeOther)
	})
	router.Get("/api/search", srv.handleSearch)
	router.Get("/api/embedding-jobs/{id}", srv.handleJobStatus)

	log.Printf("listening on %s", port)
	return http.ListenAndServe(port, router)
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		respondError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"))

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	params := retrieval.QueryParams{
		Query: query,
		Limit: limit,
	}

	if session, ok := contextutil.Session(ctx); ok {
		params.TenantID = session.TenantID
	}

	if s.guardrailsEnabled {
		qp := r.URL.Query()

		if value, provided, err := parseOptionalBoolParam(qp.Get("includeSuperseded")); err != nil {
			log.Printf("search: invalid includeSuperseded value: %v", err)
			respondError(w, http.StatusBadRequest, "invalid includeSuperseded value")
			return
		} else if provided && value {
			if !contextutil.CanIncludeSuperseded(ctx) {
				respondError(w, http.StatusForbidden, "includeSuperseded requires elevated permission")
				return
			}
			params.IncludeSuperseded = true
		}

		if value, provided, err := parseOptionalBoolParam(qp.Get("includeInterpretive")); err != nil {
			log.Printf("search: invalid includeInterpretive value: %v", err)
			respondError(w, http.StatusBadRequest, "invalid includeInterpretive value")
			return
		} else if provided && value {
			if !contextutil.CanIncludeInterpretive(ctx) {
				respondError(w, http.StatusForbidden, "includeInterpretive requires elevated permission")
				return
			}
			params.IncludeInterpretive = true
		}

		if value, provided, err := parseOptionalBoolParam(qp.Get("includeInternal")); err != nil {
			log.Printf("search: invalid includeInternal value: %v", err)
			respondError(w, http.StatusBadRequest, "invalid includeInternal value")
			return
		} else if provided && value {
			if !contextutil.CanIncludeInternal(ctx) {
				respondError(w, http.StatusForbidden, "includeInternal requires elevated permission")
				return
			}
			if params.TenantID == uuid.Nil {
				respondError(w, http.StatusBadRequest, "tenant context required for internal content")
				return
			}
			params.IncludeInternal = true
		}

		if rawAsOf := strings.TrimSpace(qp.Get("asOf")); rawAsOf != "" {
			if !contextutil.CanUseAsOf(ctx) {
				respondError(w, http.StatusForbidden, "asOf parameter requires elevated permission")
				return
			}
			asOf, err := parseAsOfValue(rawAsOf)
			if err != nil {
				log.Printf("search: invalid asOf value: %v", err)
				respondError(w, http.StatusBadRequest, "invalid asOf value")
				return
			}
			params.AsOf = &asOf
		}
	} else {
		// Guardrails disabled: reject guardrail-specific query params to avoid ambiguous behaviour.
		if hasGuardrailParam(r.URL.Query()) {
			respondError(w, http.StatusBadRequest, "guardrail parameters are unavailable while guardrails are disabled")
			return
		}
	}

	results, err := s.retrieval.Search(ctx, params)
	if err != nil {
		if errors.Is(err, retrieval.ErrTenantRequired) {
			respondError(w, http.StatusBadRequest, "tenant context required for internal content")
			return
		}
		log.Printf("retrieval error: %v", err)
		respondError(w, http.StatusInternalServerError, "search failed")
		return
	}

	resp := searchResponse{Results: make([]searchResult, 0, len(results))}
	for _, result := range results {
		resp.Results = append(resp.Results, searchResult{
			Reference: result.ASCReference,
			Score:     result.Score,
			Excerpt:   buildExcerpt(result.Content),
			Content:   result.Content,
		})
	}

	respondJSON(w, http.StatusOK, resp)
}

func (s *server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		respondError(w, http.StatusInternalServerError, "database unavailable")
		return
	}

	idParam := chi.URLParam(r, "id")
	jobID, err := uuid.Parse(strings.TrimSpace(idParam))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var resp embeddingJobStatus
	var lastErr sql.NullString
	var completedAt sql.NullTime
	var paragraphStatus sql.NullString
	row := s.db.QueryRowContext(ctx, `
		select
			j.id,
			j.paragraph_id,
			j.status,
			j.attempts,
			j.last_error,
			j.created_at,
			j.updated_at,
			j.completed_at,
			p.embedding_status
		from embedding_jobs j
		left join asc_paragraphs p on p.id = j.paragraph_id
		where j.id = $1
	`, jobID)

	err = row.Scan(
		&resp.JobID,
		&resp.ParagraphID,
		&resp.Status,
		&resp.Attempts,
		&lastErr,
		&resp.CreatedAt,
		&resp.UpdatedAt,
		&completedAt,
		&paragraphStatus,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "job not found")
			return
		}
		log.Printf("job status query failed: %v", err)
		respondError(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	if lastErr.Valid {
		resp.LastError = &lastErr.String
	}
	if completedAt.Valid {
		resp.CompletedAt = &completedAt.Time
	}
	if paragraphStatus.Valid {
		resp.ParagraphEmbeddingStatus = &paragraphStatus.String
	}

	respondJSON(w, http.StatusOK, resp)
}

func parseLimit(raw string) int {
	if raw == "" {
		return 5
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 5
	}
	if value > 25 {
		return 25
	}
	return value
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func parseBoolEnv(key string, def bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return value
}

func parseOptionalBoolParam(raw string) (bool, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, true, err
	}
	return value, true, nil
}

func parseAsOfValue(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("empty asOf value")
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("2006-01-02", raw); err == nil {
		return ts, nil
	}
	return time.Time{}, fmt.Errorf("unsupported asOf format: %s", raw)
}

func hasGuardrailParam(values url.Values) bool {
	if len(values["includeSuperseded"]) > 0 {
		return true
	}
	if len(values["includeInterpretive"]) > 0 {
		return true
	}
	if len(values["includeInternal"]) > 0 {
		return true
	}
	if len(values["asOf"]) > 0 {
		return true
	}
	return false
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write json: %v", err)
	}
}

func buildExcerpt(content string) string {
	const max = 240
	sanitised := strings.TrimSpace(content)
	if len(sanitised) <= max {
		return sanitised
	}
	return sanitised[:max] + "…"
}

type searchResponse struct {
	Results []searchResult `json:"results"`
}

type searchResult struct {
	Reference string  `json:"ref"`
	Score     float64 `json:"score"`
	Excerpt   string  `json:"excerpt"`
	Content   string  `json:"content"`
}

type embeddingJobStatus struct {
	JobID                    uuid.UUID  `json:"job_id"`
	ParagraphID              uuid.UUID  `json:"paragraph_id"`
	Status                   string     `json:"status"`
	Attempts                 int        `json:"attempts"`
	LastError                *string    `json:"last_error"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	CompletedAt              *time.Time `json:"completed_at"`
	ParagraphEmbeddingStatus *string    `json:"paragraph_embedding_status"`
}
