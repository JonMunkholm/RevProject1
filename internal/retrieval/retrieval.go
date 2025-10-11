// Package retrieval implements Stage 1 semantic search primitives.
//
// The ingest CLI populates asc_paragraphs and asc_embeddings. The search API
// will call Service.Search to rank results using pgvector cosine similarity.
// Behaviour is defined by docs/stage1_working_app_checklist.md.
package retrieval

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// QueryParams captures user input for a Stage 1 query.
type QueryParams struct {
	Query string
	Limit int
}

// Result represents a ranked hit returned to the API layer.
type Result struct {
	ParagraphID   uuid.UUID
	ASCReference  string
	Content       string
	Score         float64
	Guidance      string
	SourceType    string
	Authority     float64
	SchemaVersion string
}

// Service executes semantic search using a database handle and embedding client.
type Service struct {
	db        *sql.DB
	http      *http.Client
	openAIURL string
	openAIKey string
	model     string
	projectID string
}

// Config configures a retrieval Service.
type Config struct {
	DB        *sql.DB
	HTTP      *http.Client
	OpenAIURL string
	OpenAIKey string
	Model     string
	ProjectID string
}

// NewService builds a Service backed by the supplied components.
func NewService(cfg Config) (*Service, error) {
	if cfg.DB == nil {
		return nil, errors.New("retrieval: db handle is required")
	}
	if cfg.OpenAIKey == "" {
		return nil, errors.New("retrieval: openai api key is required")
	}
	client := cfg.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	base := cfg.OpenAIURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := cfg.Model
	if model == "" {
		model = "text-embedding-3-large"
	}
	return &Service{
		db:        cfg.DB,
		http:      client,
		openAIURL: strings.TrimRight(base, "/"),
		openAIKey: cfg.OpenAIKey,
		model:     model,
		projectID: cfg.ProjectID,
	}, nil
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// Search issues the Stage 1 cosine query for the supplied text.
func (s *Service) Search(ctx context.Context, params QueryParams) ([]Result, error) {
	if s == nil {
		return nil, errors.New("retrieval: service is nil")
	}
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return nil, errors.New("retrieval: query text is required")
	}
	limit := params.Limit
	if limit <= 0 || limit > 25 {
		limit = 5
	}

	vector, err := s.generateEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}

	vectorLiteral := formatVectorLiteral(vector)

	rows, err := s.db.QueryContext(ctx, searchSQL, vectorLiteral, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]Result, 0, limit)
	for rows.Next() {
		var r Result
		if err := rows.Scan(
			&r.ParagraphID,
			&r.ASCReference,
			&r.Content,
			&r.Score,
			&r.Guidance,
			&r.SourceType,
			&r.Authority,
			&r.SchemaVersion,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Service) generateEmbedding(ctx context.Context, input string) ([]float32, error) {
	payload := embeddingRequest{Model: s.model, Input: input}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := s.openAIURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.openAIKey)
	req.Header.Set("Content-Type", "application/json")
	if s.projectID != "" {
		req.Header.Set("OpenAI-Project", s.projectID)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai embeddings failed: %s (%s)", resp.Status, strings.TrimSpace(string(data)))
	}

	var parsed embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, errors.New("retrieval: embedding response missing vector data")
	}

	vec := make([]float32, len(parsed.Data[0].Embedding))
	for i, v := range parsed.Data[0].Embedding {
		vec[i] = float32(v)
	}
	return vec, nil
}

func formatVectorLiteral(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}
	values := make([]string, len(vec))
	for i, v := range vec {
		values[i] = fmt.Sprintf("%.8f", v)
	}
	return "[" + strings.Join(values, ",") + "]"
}

const searchSQL = `
with query as (
	select $1::vector as embedding
)
select
	p.id as paragraph_id,
	p.asc_reference,
	p.content,
	1 - (e.embedding <=> query.embedding) as score,
	p.guidance_version,
	p.source_type,
	p.authority_score,
	p.schema_version
from asc_embeddings e
join asc_paragraphs p on p.id = e.paragraph_id
join query on true
order by e.embedding <=> query.embedding
limit $2`
