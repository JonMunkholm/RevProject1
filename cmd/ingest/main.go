package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/ai/openai"
	"github.com/JonMunkholm/RevProject1/internal/embeddingjobs"
	"github.com/JonMunkholm/RevProject1/internal/stage1"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type options struct {
	FilePath        string
	DBURL           string
	QueueURL        string
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

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
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

	if strings.TrimSpace(opts.QueueURL) != "" {
		return enqueueAsyncJob(ctx, db, opts, content, sourceID)
	}

	return runSyncEmbedding(ctx, db, opts, content, sourceID)
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
	flag.StringVar(&opts.QueueURL, "queue-url", os.Getenv("EMBED_QUEUE_URL"), "Embedding job SQS queue URL (defaults to EMBED_QUEUE_URL env)")
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
				value = strings.Trim(value, " \"'")
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
	return insertParagraphExec(ctx, db, paragraphID, opts, content, sourceID)
}

func insertParagraphExec(ctx context.Context, exec sqlExecutor, paragraphID uuid.UUID, opts options, content, sourceID string) error {
	_, err := exec.ExecContext(ctx, `
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

func enqueueAsyncJob(ctx context.Context, db *sql.DB, opts options, content, sourceHash string) error {
	client, err := newSQSClient(ctx)
	if err != nil {
		return fmt.Errorf("init sqs client: %w", err)
	}

	paragraphID := uuid.New()
	jobID := uuid.New()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := insertParagraphExec(ctx, tx, paragraphID, opts, content, sourceHash); err != nil {
		return fmt.Errorf("insert paragraph: %w", err)
	}

	if err := insertEmbeddingJob(ctx, tx, jobID, paragraphID, opts, sourceHash); err != nil {
		return fmt.Errorf("insert embedding job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit paragraph/job transaction: %w", err)
	}

	message := embeddingjobs.Message{
		JobID:           jobID,
		ParagraphID:     paragraphID,
		SourceHash:      sourceHash,
		Model:           opts.Model,
		Priority:        "normal",
		MetadataVersion: opts.SchemaVersion,
		CreatedAt:       time.Now().UTC(),
	}

	if err := enqueueEmbeddingJob(ctx, client, opts.QueueURL, message); err != nil {
		updateParagraphStatus(ctx, db, paragraphID, "failed")
		markJobFailed(ctx, db, jobID, err)
		return fmt.Errorf("enqueue embedding job: %w", err)
	}

	fmt.Printf("Queued embedding job %s for paragraph %s\n", jobID, paragraphID)
	return nil
}

func runSyncEmbedding(ctx context.Context, db *sql.DB, opts options, content, sourceHash string) error {
	paragraphID := uuid.New()
	if err := insertParagraph(ctx, db, paragraphID, opts, content, sourceHash); err != nil {
		return fmt.Errorf("insert paragraph: %w", err)
	}

	if err := updateParagraphStatus(ctx, db, paragraphID, "processing"); err != nil {
		return fmt.Errorf("set paragraph status: %w", err)
	}

	embedding, modelUsed, err := openai.GenerateEmbedding(ctx, opts.OpenAIBase, opts.OpenAIKey, opts.OpenAIProject, opts.Model, content)
	if err != nil {
		updateParagraphStatus(ctx, db, paragraphID, "failed")
		return fmt.Errorf("generate embedding: %w", err)
	}

	embeddingID := uuid.New()
	if err := insertEmbedding(ctx, db, embeddingID, paragraphID, embedding, modelUsed, opts); err != nil {
		updateParagraphStatus(ctx, db, paragraphID, "failed")
		return fmt.Errorf("insert embedding: %w", err)
	}

	if err := updateParagraphStatus(ctx, db, paragraphID, "succeeded"); err != nil {
		return fmt.Errorf("set paragraph succeeded: %w", err)
	}

	fmt.Printf("Ingested paragraph %s with embedding %s (%d dimensions)\n", paragraphID, modelUsed, len(embedding))
	return nil
}

func insertEmbeddingJob(ctx context.Context, exec sqlExecutor, jobID, paragraphID uuid.UUID, opts options, sourceHash string) error {
	_, err := exec.ExecContext(ctx, `
		insert into embedding_jobs (
			id,
			paragraph_id,
			source_hash,
			model,
			priority,
			metadata_version
		) values ($1,$2,$3,$4,$5,$6)`,
		jobID,
		paragraphID,
		sourceHash,
		opts.Model,
		"normal",
		opts.SchemaVersion,
	)
	return err
}

func enqueueEmbeddingJob(ctx context.Context, client *sqs.Client, queueURL string, msg embeddingjobs.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(body)),
	})
	return err
}

func newSQSClient(ctx context.Context) (*sqs.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return sqs.NewFromConfig(cfg), nil
}

func updateParagraphStatus(ctx context.Context, exec sqlExecutor, paragraphID uuid.UUID, status string) error {
	_, err := exec.ExecContext(ctx, `update asc_paragraphs set embedding_status = $2, updated_at = now() where id = $1`, paragraphID, status)
	return err
}

func markJobFailed(ctx context.Context, db *sql.DB, jobID uuid.UUID, reason error) {
	_, _ = db.ExecContext(ctx, `
		update embedding_jobs
		set status = 'failed',
		    last_error = $2,
		    completed_at = now(),
		    updated_at = now()
		where id = $1`, jobID, reason.Error())
}
