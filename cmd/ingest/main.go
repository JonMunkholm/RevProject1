package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/stage1"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type options struct {
	FilePath        string
	DBURL           string
	Framework       string
	Topic           string
	Reference       string
	GuidanceVersion string
	SchemaVersion   string
	SourceType      string
	AuthorityScore  float64
	IndexRole       string
	Model           string
	CreatedBy       string
	OpenAIBase      string
	OpenAIKey       string
	OpenAIProject   string
}

type embeddingResponse struct {
	Model string `json:"model"`
	Data  []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func main() {
	log.SetFlags(0)
	if err := run(context.Background()); err != nil {
		log.Fatalf("ingest: %v", err)
	}
}

func run(ctx context.Context) error {
	_ = godotenv.Load()

	opts, err := parseOptions()
	if err != nil {
		return err
	}

	content, meta, err := extractContent(opts.FilePath)
	if err != nil {
		return fmt.Errorf("read paragraph: %w", err)
	}
	opts.applyMetadata(meta)
	if err := opts.validate(); err != nil {
		return err
	}

	hash := sha256.Sum256([]byte(content))
	sourceID := hex.EncodeToString(hash[:])

	db, err := sql.Open("postgres", opts.DBURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(4)
	db.SetMaxOpenConns(4)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	if err := stage1.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}

	if existingID, err := lookupParagraphBySource(ctx, db, sourceID); err == nil {
		log.Printf("paragraph already ingested (id=%s) – skipping re-embed", existingID)
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check existing paragraph: %w", err)
	}

	paragraphID := uuid.New()
	if err := insertParagraph(ctx, db, paragraphID, opts, content, sourceID); err != nil {
		return fmt.Errorf("insert paragraph: %w", err)
	}

	embedding, modelUsed, err := generateEmbedding(ctx, opts.OpenAIBase, opts.OpenAIKey, opts.OpenAIProject, opts.Model, content)
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}

	embeddingID := uuid.New()
	if err := insertEmbedding(ctx, db, embeddingID, paragraphID, embedding, modelUsed, opts); err != nil {
		return fmt.Errorf("insert embedding: %w", err)
	}

	fmt.Printf("Ingested paragraph %s with embedding %s (%d dimensions)\n", paragraphID, modelUsed, len(embedding))
	return nil
}

func parseOptions() (options, error) {
	opts := options{
		Framework:       "US_GAAP",
		Topic:           "ASC606",
		GuidanceVersion: "ASU2014-09",
		SchemaVersion:   "v1.0-2025-10-15",
		SourceType:      "authoritative",
		AuthorityScore:  1.0,
		IndexRole:       "authoritative_current",
		Model:           "text-embedding-3-large",
		CreatedBy:       "cmd/ingest",
		OpenAIBase:      "https://api.openai.com/v1",
	}

	flag.StringVar(&opts.FilePath, "file", "", "Path to the paragraph text file to ingest")
	flag.StringVar(&opts.DBURL, "db", "", "Postgres connection string (defaults to DB_URL env)")
	flag.StringVar(&opts.Framework, "framework", opts.Framework, "Accounting framework (e.g. US_GAAP)")
	flag.StringVar(&opts.Topic, "topic", opts.Topic, "Topic identifier (e.g. ASC606)")
	flag.StringVar(&opts.Reference, "reference", opts.Reference, "ASC reference (e.g. ASC606-10-25-1)")
	flag.StringVar(&opts.GuidanceVersion, "guidance-version", opts.GuidanceVersion, "Guidance version (e.g. ASU2014-09)")
	flag.StringVar(&opts.SchemaVersion, "schema-version", opts.SchemaVersion, "Schema version tag")
	flag.StringVar(&opts.SourceType, "source-type", opts.SourceType, "Source type (authoritative|interpretive|internal)")
	flag.Float64Var(&opts.AuthorityScore, "authority-score", opts.AuthorityScore, "Authority score for ranking")
	flag.StringVar(&opts.IndexRole, "index-role", opts.IndexRole, "Embedding index role")
	flag.StringVar(&opts.Model, "model", opts.Model, "Embedding model identifier")
	flag.StringVar(&opts.CreatedBy, "created-by", opts.CreatedBy, "Created_by column value for embeddings")
	flag.StringVar(&opts.OpenAIBase, "openai-base", opts.OpenAIBase, "OpenAI base URL (defaults to https://api.openai.com/v1)")
	flag.Parse()

	if opts.FilePath == "" {
		return options{}, errors.New("file path is required (use -file)")
	}

	if opts.DBURL == "" {
		opts.DBURL = os.Getenv("DB_URL")
	}
	if opts.DBURL == "" {
		return options{}, errors.New("db connection string not provided (set DB_URL or use -db)")
	}

	opts.OpenAIKey = os.Getenv("OPENAI_API_KEY")
	if opts.OpenAIKey == "" {
		return options{}, errors.New("OPENAI_API_KEY must be set")
	}
	opts.OpenAIProject = os.Getenv("OPENAI_PROJECT_ID")

	return opts, nil
}

func (o *options) applyMetadata(meta map[string]string) {
	if v, ok := meta["framework"]; ok && o.Framework == "" {
		o.Framework = v
	}
	if v, ok := meta["topic"]; ok && o.Topic == "" {
		o.Topic = v
	}
	if v, ok := meta["asc_reference"]; ok && o.Reference == "" {
		o.Reference = v
	}
	if v, ok := meta["guidance_version"]; ok && o.GuidanceVersion == "" {
		o.GuidanceVersion = v
	}
	if v, ok := meta["schema_version"]; ok && o.SchemaVersion == "" {
		o.SchemaVersion = v
	}
}

func (o options) validate() error {
	if o.Reference == "" {
		return errors.New("asc reference is required (use -reference or specify in file header)")
	}
	if o.SchemaVersion == "" {
		return errors.New("schema version is required")
	}
	return nil
}

func extractContent(path string) (string, map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()

	meta := make(map[string]string)
	var bodyLines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			metaLine := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if metaLine == "" {
				continue
			}
			if parts := strings.SplitN(metaLine, ":", 2); len(parts) == 2 {
				key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(parts[0]), " ", "_"))
				value := strings.TrimSpace(parts[1])
				if key != "" && value != "" {
					meta[key] = value
				}
			} else if strings.HasPrefix(metaLine, "ASC") && meta["asc_reference"] == "" {
				meta["asc_reference"] = metaLine
			}
			continue
		}
		bodyLines = append(bodyLines, line)
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}

	content := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if content == "" {
		return "", nil, errors.New("no paragraph content detected in file")
	}
	return content, meta, nil
}

func lookupParagraphBySource(ctx context.Context, db *sql.DB, sourceID string) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRowContext(ctx, `select id from asc_paragraphs where source_id = $1 limit 1`, sourceID).Scan(&id)
	return id, err
}

func insertParagraph(ctx context.Context, db *sql.DB, paragraphID uuid.UUID, opts options, content, sourceID string) error {
	_, err := db.ExecContext(ctx, `
		insert into asc_paragraphs (
			id,
			framework,
			topic,
			asc_reference,
			guidance_version,
			source_type,
			authority_score,
			source_id,
			schema_version,
			content
		)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		paragraphID,
		opts.Framework,
		opts.Topic,
		opts.Reference,
		opts.GuidanceVersion,
		opts.SourceType,
		opts.AuthorityScore,
		sourceID,
		opts.SchemaVersion,
		content,
	)
	return err
}

func insertEmbedding(ctx context.Context, db *sql.DB, embeddingID, paragraphID uuid.UUID, embedding []float32, modelUsed string, opts options) error {
	vectorLiteral := formatVectorLiteral(embedding)
	_, err := db.ExecContext(ctx, `
		insert into asc_embeddings (
			id,
			paragraph_id,
			embedding,
			embedding_model,
			embedding_date,
			index_role,
			schema_version,
			created_by
		)
		values ($1,$2,$3::vector,$4,$5,$6,$7,$8)`,
		embeddingID,
		paragraphID,
		vectorLiteral,
		modelUsed,
		time.Now().UTC(),
		opts.IndexRole,
		opts.SchemaVersion,
		opts.CreatedBy,
	)
	return err
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

func generateEmbedding(ctx context.Context, baseURL, apiKey, projectID, model, input string) ([]float32, string, error) {
	payload := map[string]any{
		"model": model,
		"input": input,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if projectID != "" {
		req.Header.Set("OpenAI-Project", projectID)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("openai embeddings failed: %s (%s)", resp.Status, strings.TrimSpace(string(data)))
	}

	var parsed embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, "", err
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, "", errors.New("embedding response missing vector data")
	}

	vector := make([]float32, len(parsed.Data[0].Embedding))
	for i, v := range parsed.Data[0].Embedding {
		vector[i] = float32(v)
	}

	modelUsed := parsed.Model
	if modelUsed == "" {
		modelUsed = model
	}
	return vector, modelUsed, nil
}
