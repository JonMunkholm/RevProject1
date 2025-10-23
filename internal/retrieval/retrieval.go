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
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// QueryParams captures user input and guardrail selections for a search request.
type QueryParams struct {
	Query               string
	Limit               int
	IncludeSuperseded   bool
	IncludeInterpretive bool
	IncludeInternal     bool
	AsOf                *time.Time
	TenantID            uuid.UUID
}

var ErrTenantRequired = errors.New("retrieval: tenant context required for internal content")

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
	db                *sql.DB
	http              *http.Client
	openAIURL         string
	openAIKey         string
	model             string
	projectID         string
	guardrailsEnabled bool
}

// Config configures a retrieval Service.
type Config struct {
	DB                *sql.DB
	HTTP              *http.Client
	OpenAIURL         string
	OpenAIKey         string
	Model             string
	ProjectID         string
	GuardrailsEnabled bool
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
		db:                cfg.DB,
		http:              client,
		openAIURL:         strings.TrimRight(base, "/"),
		openAIKey:         cfg.OpenAIKey,
		model:             model,
		projectID:         cfg.ProjectID,
		guardrailsEnabled: cfg.GuardrailsEnabled,
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
func (s *Service) Search(ctx context.Context, params QueryParams) (results []Result, err error) {
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

	if s.guardrailsEnabled {
		if params.IncludeInternal && params.TenantID == uuid.Nil {
			return nil, ErrTenantRequired
		}
	}

	vector, err := s.generateEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}

	vectorLiteral := formatVectorLiteral(vector)

	args := []interface{}{vectorLiteral, limit}
	sqlText := searchLegacySQL

	var (
		runner queryer = s.db
		tx     *sql.Tx
	)

	if s.guardrailsEnabled {
		tx, err = s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return nil, err
		}
		runner = tx

		if err = s.configureGuardrailSession(ctx, tx, params); err != nil {
			_ = tx.Rollback()
			return nil, err
		}

		sqlText, args, err = s.buildGuardrailSQL(vectorLiteral, limit, params)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}

		defer func() {
			if tx == nil {
				return
			}
			if err != nil {
				_ = tx.Rollback()
				return
			}
			if commitErr := tx.Commit(); commitErr != nil {
				err = commitErr
			}
		}()
	}

	rows, err := runner.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results = make([]Result, 0, limit)
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

type execer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

func (s *Service) configureGuardrailSession(ctx context.Context, exec execer, params QueryParams) error {
	boolStr := func(v bool) string {
		return strconv.FormatBool(v)
	}

	settings := []struct {
		key   string
		value string
	}{
		{"app.guardrails.enabled", "true"},
		{"app.guardrails.include_superseded", boolStr(params.IncludeSuperseded)},
		{"app.guardrails.include_interpretive", boolStr(params.IncludeInterpretive)},
		{"app.guardrails.include_internal", boolStr(params.IncludeInternal)},
	}

	tenantValue := ""
	if params.TenantID != uuid.Nil {
		tenantValue = params.TenantID.String()
	}
	settings = append(settings, struct {
		key   string
		value string
	}{"app.guardrails.tenant_id", tenantValue})

	asOfValue := ""
	if params.AsOf != nil {
		asOfValue = params.AsOf.UTC().Format(time.RFC3339)
	}
	settings = append(settings, struct {
		key   string
		value string
	}{"app.guardrails.as_of", asOfValue})

	for _, setting := range settings {
		if _, err := exec.ExecContext(ctx, "select set_config($1, $2, true)", setting.key, setting.value); err != nil {
			return fmt.Errorf("set_config %s: %w", setting.key, err)
		}
	}

	return nil
}

func (s *Service) buildGuardrailSQL(vectorLiteral string, limit int, params QueryParams) (string, []interface{}, error) {
	args := []interface{}{vectorLiteral, limit}
	filters := make([]string, 0, 6)
	next := 3

	if !params.IncludeSuperseded {
		filters = append(filters, "p.superseded = false")
	}

	if params.AsOf != nil {
		asOf := params.AsOf.UTC()
		filters = append(filters, fmt.Sprintf("(p.effective_date IS NULL OR p.effective_date <= $%d)", next))
		args = append(args, asOf)
		next++
	} else {
		filters = append(filters, "(p.effective_date IS NULL OR p.effective_date <= now())")
	}

	sourceTypes := []string{"authoritative"}
	if params.IncludeInterpretive {
		sourceTypes = append(sourceTypes, "interpretive")
	}
	if params.IncludeInternal {
		sourceTypes = append(sourceTypes, "internal")
	}

	if len(sourceTypes) == 1 {
		filters = append(filters, "p.source_type = 'authoritative'")
	} else {
		filters = append(filters, fmt.Sprintf("p.source_type = ANY($%d)", next))
		args = append(args, pq.StringArray(sourceTypes))
		next++
	}

	if params.IncludeInternal {
		filters = append(filters, fmt.Sprintf("(p.tenant_id IS NULL OR p.tenant_id = $%d)", next))
		args = append(args, params.TenantID)
		next++
	} else {
		filters = append(filters, "(p.tenant_id IS NULL)")
	}

	var builder strings.Builder
	builder.WriteString(guardrailSelectClause)
	if len(filters) > 0 {
		builder.WriteString("WHERE ")
		builder.WriteString(strings.Join(filters, "\n  AND "))
		builder.WriteString("\n")
	}
	builder.WriteString(guardrailOrderClause)

	return builder.String(), args, nil
}

const searchLegacySQL = `
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

const guardrailSelectClause = `
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
`

const guardrailOrderClause = `
order by e.embedding <=> query.embedding
limit $2`
