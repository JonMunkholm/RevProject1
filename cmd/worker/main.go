package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/ai/openai"
	"github.com/JonMunkholm/RevProject1/internal/awsutil"
	"github.com/JonMunkholm/RevProject1/internal/embeddingjobs"
	"github.com/JonMunkholm/RevProject1/internal/stage1"
	"github.com/aws/aws-sdk-go-v2/aws"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type options struct {
	DBURL         string
	QueueURL      string
	DLQURL        string
	OpenAIBase    string
	OpenAIKey     string
	OpenAIProject string
	IndexRole     string
	CreatedBy     string
	WaitSeconds   int32
	MaxMessages   int32
	MetricsAddr   string
	MaxAttempts   int
	CWNamespace   string
	CWEnabled     bool
	Environment   string
}

func main() {
	log.SetFlags(0)
	if err := run(context.Background()); err != nil {
		log.Fatalf("worker: %v", err)
	}
}

func run(ctx context.Context) error {
	_ = godotenv.Load()

	opts, err := parseOptions()
	if err != nil {
		return err
	}

	db, err := sql.Open("postgres", opts.DBURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxIdleConns(4)
	db.SetMaxOpenConns(8)

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	if err := stage1.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}

	client, err := awsutil.NewSQSClient(ctx)
	if err != nil {
		return fmt.Errorf("init sqs client: %w", err)
	}

	promMetrics := newWorkerMetrics()

	var cwPublisher *awsutil.CloudWatchPublisher
	if opts.CWEnabled {
		dims := map[string]string{
			"Service":     "embedding_worker",
			"Environment": opts.Environment,
		}
		pub, err := awsutil.NewCloudWatchPublisher(ctx, opts.CWNamespace, dims)
		if err != nil {
			log.Printf("cloudwatch init error: %v (metrics publishing disabled)", err)
		} else {
			cwPublisher = pub
		}
	}

	worker := &embeddingWorker{
		db:       db,
		client:   client,
		opts:     opts,
		metrics:  newMetricsReporter(promMetrics, cwPublisher),
		visGrace: 15 * time.Second,
	}
	worker.baseVis = clampInt32(opts.WaitSeconds+int32(worker.visGrace.Seconds()), 30, 600)

	log.Printf("worker started; polling queue %s", opts.QueueURL)
	log.Printf("using OpenAI base %q", opts.OpenAIBase)
	return worker.loop(ctx)
}

func parseOptions() (options, error) {
	metricsAddr := os.Getenv("WORKER_METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = ":2112"
	}

	openAIBase := os.Getenv("OPENAI_API_BASE")
	if openAIBase == "" {
		openAIBase = "https://api.openai.com/v1"
	}

	cwNamespace := os.Getenv("CLOUDWATCH_NAMESPACE")
	if strings.TrimSpace(cwNamespace) == "" {
		cwNamespace = "EGRA/EmbeddingWorker"
	}

	cwEnabled := true
	if raw := strings.TrimSpace(os.Getenv("CLOUDWATCH_ENABLED")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			cwEnabled = parsed
		}
	}

	envName := strings.TrimSpace(os.Getenv("ENVIRONMENT"))
	if envName == "" {
		envName = "local"
	}

	dlqURL := os.Getenv("EMBED_DLQ_URL")

	maxAttempts := 3
	if raw := strings.TrimSpace(os.Getenv("EMBED_MAX_ATTEMPTS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			maxAttempts = parsed
		}
	}

	opts := options{
		QueueURL:    os.Getenv("EMBED_QUEUE_URL"),
		DBURL:       os.Getenv("DB_URL"),
		OpenAIBase:  openAIBase,
		IndexRole:   "authoritative_current",
		CreatedBy:   "worker/embedding",
		WaitSeconds: 20,
		MaxMessages: 5,
		MetricsAddr: metricsAddr,
		DLQURL:      dlqURL,
		MaxAttempts: maxAttempts,
		CWNamespace: cwNamespace,
		CWEnabled:   cwEnabled,
		Environment: envName,
	}

	flag.StringVar(&opts.QueueURL, "queue-url", opts.QueueURL, "Embedding job SQS queue URL (required)")
	flag.StringVar(&opts.DBURL, "db", opts.DBURL, "Database DSN (defaults to DB_URL env)")
	flag.StringVar(&opts.OpenAIBase, "openai-base", opts.OpenAIBase, "OpenAI base URL")
	flag.StringVar(&opts.OpenAIProject, "openai-project", os.Getenv("OPENAI_PROJECT_ID"), "OpenAI project ID")
	flag.StringVar(&opts.IndexRole, "index-role", opts.IndexRole, "Embedding index role")
	flag.StringVar(&opts.CreatedBy, "created-by", opts.CreatedBy, "created_by value for embeddings")
	flag.StringVar(&opts.DLQURL, "dlq-url", opts.DLQURL, "Dead-letter SQS queue URL (optional)")
	wait := int(opts.WaitSeconds)
	maxMsgs := int(opts.MaxMessages)
	maxAttemptsFlag := opts.MaxAttempts
	flag.IntVar(&wait, "wait", wait, "SQS long poll wait time (seconds)")
	flag.IntVar(&maxMsgs, "max-messages", maxMsgs, "Max messages per poll (1-10)")
	flag.IntVar(&maxAttemptsFlag, "max-attempts", maxAttemptsFlag, "Max attempts before a job is sent to DLQ")
	flag.StringVar(&opts.MetricsAddr, "metrics-addr", opts.MetricsAddr, "address for Prometheus metrics server (empty to disable)")
	flag.StringVar(&opts.CWNamespace, "cloudwatch-namespace", opts.CWNamespace, "CloudWatch metrics namespace")
	flag.BoolVar(&opts.CWEnabled, "cloudwatch-enabled", opts.CWEnabled, "Enable CloudWatch metric publishing")
	flag.StringVar(&opts.Environment, "environment", opts.Environment, "Environment label for metrics (defaults to ENVIRONMENT env var)")
	flag.Parse()

	opts.OpenAIKey = os.Getenv("OPENAI_API_KEY")

	if strings.TrimSpace(opts.QueueURL) == "" {
		return options{}, errors.New("queue url must be provided (set EMBED_QUEUE_URL or --queue-url)")
	}
	if strings.TrimSpace(opts.DBURL) == "" {
		return options{}, errors.New("database url must be provided (set DB_URL or --db)")
	}
	if opts.OpenAIKey == "" {
		return options{}, errors.New("OPENAI_API_KEY must be set")
	}
	if maxMsgs < 1 {
		maxMsgs = 1
	}
	if maxMsgs > 10 {
		maxMsgs = 10
	}
	if wait < 0 {
		wait = 0
	}
	if wait > 20 {
		wait = 20
	}
	opts.MaxMessages = int32(maxMsgs)
	opts.WaitSeconds = int32(wait)
	opts.CWNamespace = strings.TrimSpace(opts.CWNamespace)
	if opts.CWNamespace == "" {
		opts.CWNamespace = cwNamespace
	}
	opts.Environment = strings.TrimSpace(opts.Environment)
	if opts.Environment == "" {
		opts.Environment = envName
	}
	if maxAttemptsFlag < 1 {
		maxAttemptsFlag = 1
	}
	opts.MaxAttempts = maxAttemptsFlag

	return opts, nil
}

type embeddingWorker struct {
	db       *sql.DB
	client   *sqs.Client
	opts     options
	metrics  *metricsReporter
	visGrace time.Duration
	baseVis  int32
}

func (w *embeddingWorker) loop(ctx context.Context) error {
	if w.metrics != nil && strings.TrimSpace(w.opts.MetricsAddr) != "" {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())

			listener, err := net.Listen("tcp", w.opts.MetricsAddr)
			if err != nil {
				log.Printf("metrics server listen error: %v", err)
				return
			}
			log.Printf("metrics server listening on %s", listener.Addr().String())

			server := &http.Server{
				Handler: mux,
			}
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("metrics server error: %v", err)
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := w.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(w.opts.QueueURL),
			MaxNumberOfMessages: w.opts.MaxMessages,
			WaitTimeSeconds:     w.opts.WaitSeconds,
		})
		if err != nil {
			log.Printf("sqs receive error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if len(resp.Messages) == 0 {
			continue
		}

		for _, msg := range resp.Messages {
			if err := w.handleMessage(ctx, msg); err != nil {
				log.Printf("message processing failed: %v", err)
			}
		}
	}
}

func (w *embeddingWorker) handleMessage(ctx context.Context, msg sqstypes.Message) error {
	if msg.Body == nil {
		return errors.New("message missing body")
	}

	var payload embeddingjobs.Message
	if err := json.Unmarshal([]byte(*msg.Body), &payload); err != nil {
		log.Printf("invalid message payload: %v", err)
		w.deleteMessage(ctx, msg)
		return nil
	}

	jobStart := time.Now()
	jobCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	paragraphID, status, attempt, err := w.markJobInProgress(jobCtx, payload.JobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("job %s not found, discarding message", payload.JobID)
			w.deleteMessage(ctx, msg)
			return nil
		}
		return fmt.Errorf("mark job in progress: %w", err)
	}

	switch status {
	case "succeeded":
		if err := w.deleteMessage(ctx, msg); err != nil {
			log.Printf("delete message after completed job error: %v", err)
		}
		return nil
	case "dead_letter":
		if err := w.routeToDLQ(ctx, msg, payload, attempt, nil); err != nil {
			return fmt.Errorf("route to dlq: %w", err)
		}
		if err := w.deleteMessage(ctx, msg); err != nil {
			log.Printf("delete message after dlq error: %v", err)
		}
		return nil
	}

	w.extendVisibility(ctx, msg)

	content, err := w.fetchParagraphContent(jobCtx, paragraphID)
	if err != nil {
		deadLetter, failErr := w.failJob(jobCtx, payload.JobID, paragraphID, attempt, err)
		if failErr != nil {
			return fmt.Errorf("fetch paragraph: %w", failErr)
		}
		if deadLetter {
			if err := w.routeToDLQ(ctx, msg, payload, attempt, err); err != nil {
				return fmt.Errorf("route to dlq after fetch failure: %w", err)
			}
			if err := w.deleteMessage(ctx, msg); err != nil {
				log.Printf("delete message after fetch failure dlq error: %v", err)
			}
			w.metrics.RecordFailure(jobCtx, "dead_letter")
			w.metrics.RecordJobDuration(jobCtx, "dead_letter", time.Since(jobStart))
			return nil
		}
		w.metrics.RecordFailure(jobCtx, "failed")
		w.metrics.RecordJobDuration(jobCtx, "failed", time.Since(jobStart))
		return fmt.Errorf("fetch paragraph: %w", err)
	}

	start := time.Now()
	vector, modelUsed, err := openai.GenerateEmbedding(jobCtx, w.opts.OpenAIBase, w.opts.OpenAIKey, w.opts.OpenAIProject, payload.Model, content)
	if err != nil {
		deadLetter, failErr := w.failJob(jobCtx, payload.JobID, paragraphID, attempt, err)
		if failErr != nil {
			return fmt.Errorf("generate embedding: %w", failErr)
		}
		w.metrics.RecordOpenAILatency(jobCtx, "error", time.Since(start))
		if deadLetter {
			if err := w.routeToDLQ(ctx, msg, payload, attempt, err); err != nil {
				return fmt.Errorf("route to dlq after embedding failure: %w", err)
			}
			if err := w.deleteMessage(ctx, msg); err != nil {
				log.Printf("delete message after embedding failure dlq error: %v", err)
			}
			w.metrics.RecordFailure(jobCtx, "dead_letter")
			w.metrics.RecordJobDuration(jobCtx, "dead_letter", time.Since(jobStart))
			return nil
		}
		w.metrics.RecordFailure(jobCtx, "failed")
		w.metrics.RecordJobDuration(jobCtx, "failed", time.Since(jobStart))
		return fmt.Errorf("generate embedding: %w", err)
	}

	w.metrics.RecordOpenAILatency(jobCtx, "success", time.Since(start))

	if err := w.persistEmbedding(jobCtx, payload, paragraphID, vector, modelUsed); err != nil {
		deadLetter, failErr := w.failJob(jobCtx, payload.JobID, paragraphID, attempt, err)
		if failErr != nil {
			return fmt.Errorf("persist embedding: %w", failErr)
		}
		if deadLetter {
			if err := w.routeToDLQ(ctx, msg, payload, attempt, err); err != nil {
				return fmt.Errorf("route to dlq after persist failure: %w", err)
			}
			if err := w.deleteMessage(ctx, msg); err != nil {
				log.Printf("delete message after persist failure dlq error: %v", err)
			}
			w.metrics.RecordFailure(jobCtx, "dead_letter")
			w.metrics.RecordJobDuration(jobCtx, "dead_letter", time.Since(jobStart))
			return nil
		}
		w.metrics.RecordFailure(jobCtx, "failed")
		w.metrics.RecordJobDuration(jobCtx, "failed", time.Since(jobStart))
		return fmt.Errorf("persist embedding: %w", err)
	}

	if err := w.markJobSucceeded(jobCtx, payload.JobID, paragraphID); err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	if err := w.deleteMessage(ctx, msg); err != nil {
		log.Printf("delete message error: %v", err)
	}

	w.metrics.RecordSuccess(jobCtx)
	w.metrics.RecordJobDuration(jobCtx, "success", time.Since(jobStart))

	return nil
}

func (w *embeddingWorker) markJobInProgress(ctx context.Context, jobID uuid.UUID) (uuid.UUID, string, int, error) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, "", 0, err
	}

	defer tx.Rollback()

	var status string
	var attempts int
	var paragraphID uuid.UUID
	err = tx.QueryRowContext(ctx, `
		select status, attempts, paragraph_id
		from embedding_jobs
		where id = $1
		for update
	`, jobID).Scan(&status, &attempts, &paragraphID)
	if err != nil {
		return uuid.Nil, "", 0, err
	}

	switch status {
	case "succeeded", "dead_letter":
		return paragraphID, status, attempts, nil
	}

	if attempts >= w.opts.MaxAttempts && w.opts.MaxAttempts > 0 {
		if _, err := tx.ExecContext(ctx, `
			update embedding_jobs
			set status = 'dead_letter',
			    completed_at = now(),
			    updated_at = now()
			where id = $1
		`, jobID); err != nil {
			return uuid.Nil, "", 0, err
		}
		if err := updateParagraphStatus(ctx, tx, paragraphID, "failed"); err != nil {
			return uuid.Nil, "", 0, err
		}
		if err := tx.Commit(); err != nil {
			return uuid.Nil, "", 0, err
		}
		log.Printf("job %s exhausted max attempts (%d); marking dead_letter", jobID, attempts)
		w.metrics.RecordFailure(ctx, "dead_letter")
		return paragraphID, "dead_letter", attempts, nil
	}

	if _, err := tx.ExecContext(ctx, `
		update embedding_jobs
		set status = 'in_progress',
		    attempts = attempts + 1,
		    updated_at = now()
	where id = $1
	`, jobID); err != nil {
		return uuid.Nil, "", 0, err
	}
	attempts++

	if err := updateParagraphStatus(ctx, tx, paragraphID, "processing"); err != nil {
		return uuid.Nil, "", 0, err
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, "", 0, err
	}

	return paragraphID, "in_progress", attempts, nil
}

func (w *embeddingWorker) fetchParagraphContent(ctx context.Context, paragraphID uuid.UUID) (string, error) {
	var content string
	err := w.db.QueryRowContext(ctx, `select content from asc_paragraphs where id = $1`, paragraphID).Scan(&content)
	return content, err
}

func (w *embeddingWorker) persistEmbedding(ctx context.Context, payload embeddingjobs.Message, paragraphID uuid.UUID, vector []float32, modelUsed string) error {
	vectorLiteral := formatVectorLiteral(vector)

	_, err := w.db.ExecContext(ctx, `
		insert into asc_embeddings (
			id,
			paragraph_id,
			embedding,
			embedding_model,
			embedding_date,
			index_role,
			schema_version,
			created_by
		) values ($1,$2,$3::vector,$4,$5,$6,$7,$8)
	`,
		uuid.New(),
		paragraphID,
		vectorLiteral,
		modelUsed,
		time.Now().UTC(),
		w.opts.IndexRole,
		payload.MetadataVersion,
		w.opts.CreatedBy,
	)
	return err
}

func (w *embeddingWorker) markJobSucceeded(ctx context.Context, jobID, paragraphID uuid.UUID) error {
	if _, err := w.db.ExecContext(ctx, `
		update embedding_jobs
		set status = 'succeeded',
		    updated_at = now(),
		    completed_at = now(),
		    last_error = null
		where id = $1
	`, jobID); err != nil {
		return err
	}

	return updateParagraphStatus(ctx, w.db, paragraphID, "succeeded")
}

func (w *embeddingWorker) failJob(ctx context.Context, jobID, paragraphID uuid.UUID, attempt int, reason error) (bool, error) {
	msg := reason.Error()
	status := "failed"
	if w.opts.MaxAttempts > 0 && attempt >= w.opts.MaxAttempts {
		status = "dead_letter"
	}

	_, err := w.db.ExecContext(ctx, `
		update embedding_jobs
		set status = $2,
		    last_error = $3,
		    updated_at = now(),
		    completed_at = case when $2 = 'dead_letter' then now() else completed_at end
		where id = $1
	`, jobID, status, msg)
	if err != nil {
		return false, err
	}

	if err := updateParagraphStatus(ctx, w.db, paragraphID, "failed"); err != nil {
		return status == "dead_letter", err
	}

	if status == "dead_letter" {
		log.Printf("job %s moved to dead_letter after %d attempts: %v", jobID, attempt, reason)
		return true, nil
	}

	return false, nil
}

func (w *embeddingWorker) deleteMessage(ctx context.Context, msg sqstypes.Message) error {
	if msg.ReceiptHandle == nil {
		return errors.New("message missing receipt handle")
	}
	_, err := w.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(w.opts.QueueURL),
		ReceiptHandle: msg.ReceiptHandle,
	})
	return err
}

func (w *embeddingWorker) routeToDLQ(ctx context.Context, msg sqstypes.Message, payload embeddingjobs.Message, attempt int, lastErr error) error {
	dlqURL := strings.TrimSpace(w.opts.DLQURL)
	if dlqURL == "" {
		return nil
	}
	if msg.Body == nil {
		return errors.New("dlq routing: message missing body")
	}

	attributes := map[string]sqstypes.MessageAttributeValue{
		"job_attempts": {
			DataType:    aws.String("Number"),
			StringValue: aws.String(strconv.Itoa(attempt)),
		},
	}
	if lastErr != nil {
		errMsg := lastErr.Error()
		if len(errMsg) > 1024 {
			errMsg = errMsg[:1024]
		}
		attributes["job_error"] = sqstypes.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(errMsg),
		}
	}

	_, err := w.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(dlqURL),
		MessageBody:       aws.String(*msg.Body),
		MessageAttributes: attributes,
	})
	if err == nil {
		log.Printf("job %s routed to DLQ", payload.JobID)
	}
	return err
}

func (w *embeddingWorker) extendVisibility(ctx context.Context, msg sqstypes.Message) {
	if msg.ReceiptHandle == nil {
		return
	}

	timeout := w.baseVis
	if timeout < 0 {
		timeout = 30
	}

	_, err := w.client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(w.opts.QueueURL),
		ReceiptHandle:     msg.ReceiptHandle,
		VisibilityTimeout: timeout,
	})
	if err != nil {
		log.Printf("change message visibility error: %v", err)
	}
}

func formatVectorLiteral(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%.8f", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func updateParagraphStatus(ctx context.Context, exec sqlExecutor, paragraphID uuid.UUID, status string) error {
	_, err := exec.ExecContext(ctx, `
		update asc_paragraphs
		set embedding_status = $2,
		    updated_at = now()
		where id = $1
	`, paragraphID, status)
	return err
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type workerMetrics struct {
	jobsProcessed  prometheus.Counter
	jobsFailed     prometheus.Counter
	jobsDeadLetter prometheus.Counter
	openaiLatency  *prometheus.HistogramVec
}

func newWorkerMetrics() *workerMetrics {
	m := &workerMetrics{
		jobsProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "embedding_worker_jobs_processed_total",
			Help: "Total number of embedding jobs processed successfully",
		}),
		jobsFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "embedding_worker_jobs_failed_total",
			Help: "Total number of embedding jobs that failed",
		}),
		jobsDeadLetter: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "embedding_worker_jobs_dead_letter_total",
			Help: "Total number of embedding jobs moved to dead letter",
		}),
		openaiLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "embedding_worker_openai_latency_seconds",
			Help:    "Latency for OpenAI embedding requests",
			Buckets: prometheus.DefBuckets,
		}, []string{"result"}),
	}

	// Register metrics with the default registry. Log errors but don't panic
	// to allow graceful handling of duplicate registrations.
	for _, c := range []prometheus.Collector{m.jobsProcessed, m.jobsFailed, m.jobsDeadLetter, m.openaiLatency} {
		if err := prometheus.Register(c); err != nil {
			// Check if it's already registered (which is fine)
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				log.Printf("failed to register prometheus metric: %v", err)
			}
		}
	}
	return m
}

func clampInt32(val, min, max int32) int32 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

type metricsReporter struct {
	prom *workerMetrics
	cw   *awsutil.CloudWatchPublisher
}

func newMetricsReporter(prom *workerMetrics, cw *awsutil.CloudWatchPublisher) *metricsReporter {
	return &metricsReporter{prom: prom, cw: cw}
}

func (m *metricsReporter) RecordSuccess(ctx context.Context) {
	if m == nil {
		return
	}
	if m.prom != nil {
		m.prom.jobsProcessed.Inc()
	}
	m.publishCount(ctx, "JobsProcessed", nil)
}

func (m *metricsReporter) RecordFailure(ctx context.Context, status string) {
	if m == nil {
		return
	}
	if m.prom != nil {
		m.prom.jobsFailed.Inc()
		if status == "dead_letter" {
			m.prom.jobsDeadLetter.Inc()
		}
	}
	dims := map[string]string{"Result": status}
	m.publishCount(ctx, "JobsFailed", dims)
	if status == "dead_letter" {
		m.publishCount(ctx, "JobsDeadLetter", dims)
	}
}

func (m *metricsReporter) RecordJobDuration(ctx context.Context, result string, duration time.Duration) {
	if m == nil {
		return
	}
	m.publishValue(ctx, "JobDurationSeconds", duration.Seconds(), map[string]string{"Result": result}, cloudwatchtypes.StandardUnitSeconds)
}

func (m *metricsReporter) RecordOpenAILatency(ctx context.Context, result string, duration time.Duration) {
	if m == nil {
		return
	}
	if m.prom != nil {
		m.prom.openaiLatency.WithLabelValues(result).Observe(duration.Seconds())
	}
	m.publishValue(ctx, "OpenAIRequestSeconds", duration.Seconds(), map[string]string{"Result": result}, cloudwatchtypes.StandardUnitSeconds)
}

func (m *metricsReporter) publishCount(ctx context.Context, name string, dims map[string]string) {
	m.publishValue(ctx, name, 1, dims, cloudwatchtypes.StandardUnitCount)
}

func (m *metricsReporter) publishValue(ctx context.Context, name string, value float64, dims map[string]string, unit cloudwatchtypes.StandardUnit) {
	if m == nil || m.cw == nil {
		return
	}
	if err := m.cw.Publish(ctx, name, value, unit, dims); err != nil {
		log.Printf("cloudwatch publish error (%s): %v", name, err)
	}
}
