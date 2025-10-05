package documents

import (
	"context"
	"database/sql"
	"errors"
	"time"

	clientpkg "github.com/JonMunkholm/RevProject1/internal/ai/client"
)

// Processor defines the execution contract for document jobs.
type Processor interface {
	Process(ctx context.Context, job Job) (map[string]any, error)
}

// Worker coordinates background processing of queued document jobs.
type Worker struct {
	service   *Service
	processor Processor
	logger    clientpkg.Logger
	interval  time.Duration

	stop    chan struct{}
	stopped chan struct{}
}

const defaultInterval = 3 * time.Second

// NewWorker constructs a worker using the supplied service and processor.
func NewWorker(service *Service, processor Processor, logger clientpkg.Logger, interval time.Duration) *Worker {
	if logger == nil {
		logger = clientpkg.NewNoopLogger()
	}
	if interval <= 0 {
		interval = defaultInterval
	}
	return &Worker{
		service:   service,
		processor: processor,
		logger:    logger,
		interval:  interval,
		stop:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
}

// Start launches the worker loop in a separate goroutine.
func (w *Worker) Start(ctx context.Context) {
	go w.run(ctx)
}

// Stop requests the worker to halt and waits for termination.
func (w *Worker) Stop() {
	close(w.stop)
	<-w.stopped
}

// SetProcessor swaps the processor implementation used for future jobs.
func (w *Worker) SetProcessor(p Processor) {
	w.processor = p
}

func (w *Worker) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer func() {
		ticker.Stop()
		close(w.stopped)
	}()

	for {
		w.processOnce(ctx)

		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) processOnce(ctx context.Context) {
	if w.service == nil || w.processor == nil {
		return
	}

	job, err := w.service.NextQueuedJob(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return
		}
		w.logger.Error(ctx, "ai: worker failed to fetch job", err)
		return
	}

	if err := w.service.MarkStatus(ctx, job.CompanyID, job.ID, "processing", nil); err != nil {
		w.logger.Error(ctx, "ai: worker failed to mark processing", err, map[string]any{"job_id": job.ID})
		return
	}

	response, err := w.processor.Process(ctx, job)
	if err != nil {
		msg := err.Error()
		_ = w.service.MarkStatus(ctx, job.CompanyID, job.ID, "failed", &msg)
		w.logger.Error(ctx, "ai: document job failed", err, map[string]any{"job_id": job.ID})
		return
	}

	if err := w.service.Complete(ctx, job.CompanyID, job.ID, response); err != nil {
		w.logger.Error(ctx, "ai: document job completion failed", err, map[string]any{"job_id": job.ID})
	}
}
