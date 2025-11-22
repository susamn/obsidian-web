package vault

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/susamn/obsidian-web/internal/db"
	"github.com/susamn/obsidian-web/internal/explorer"
	"github.com/susamn/obsidian-web/internal/indexing"
	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/sse"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// Worker processes file events with DB-first approach
type Worker struct {
	id        int
	vaultID   string
	vaultPath string
	queue     chan syncpkg.FileChangeEvent
	dlq       chan syncpkg.FileChangeEvent // Dead letter queue
	ctx       context.Context
	wg        *sync.WaitGroup

	// Services
	dbService       *db.DBService
	indexService    *indexing.IndexService
	explorerService *explorer.ExplorerService
	sseChannel      chan sse.Event

	// Metrics
	processedCount int64
	failedCount    int64
	retriedCount   int64

	// Configuration
	maxRetries    int
	retryDelay    time.Duration
	dlqRetryDelay time.Duration
}

// NewWorker creates a new worker instance
func NewWorker(
	id int,
	vaultID string,
	vaultPath string,
	ctx context.Context,
	wg *sync.WaitGroup,
	dbService *db.DBService,
	indexService *indexing.IndexService,
	explorerService *explorer.ExplorerService,
	sseChannel chan sse.Event,
) *Worker {
	return &Worker{
		id:              id,
		vaultID:         vaultID,
		vaultPath:       vaultPath,
		queue:           make(chan syncpkg.FileChangeEvent, 1000),
		dlq:             make(chan syncpkg.FileChangeEvent, 1000),
		ctx:             ctx,
		wg:              wg,
		dbService:       dbService,
		indexService:    indexService,
		explorerService: explorerService,
		sseChannel:      sseChannel,
		maxRetries:      2,
		retryDelay:      2 * time.Second,
		dlqRetryDelay:   30 * time.Second,
	}
}

// Start starts the worker processing loop
func (w *Worker) Start() {
	w.wg.Add(1)
	go w.run()

	// Start DLQ processor
	w.wg.Add(1)
	go w.processDLQ()

	logger.WithFields(map[string]interface{}{
		"worker_id": w.id,
		"vault_id":  w.vaultID,
	}).Info("Worker started")
}

// run is the main event processing loop
func (w *Worker) run() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			logger.WithFields(map[string]interface{}{
				"worker_id": w.id,
				"vault_id":  w.vaultID,
				"processed": atomic.LoadInt64(&w.processedCount),
				"failed":    atomic.LoadInt64(&w.failedCount),
				"retried":   atomic.LoadInt64(&w.retriedCount),
			}).Info("Worker stopped")
			return

		case event, ok := <-w.queue:
			if !ok {
				logger.WithField("worker_id", w.id).Info("Worker queue closed")
				return
			}
			w.processEvent(event)
		}
	}
}

// processEvent handles a single file event
func (w *Worker) processEvent(event syncpkg.FileChangeEvent) {
	// Step 1: Update DB with retry logic
	err := w.updateDBWithRetry(event)
	if err != nil {
		// DB update failed after retries, send to DLQ
		logger.WithFields(map[string]interface{}{
			"worker_id": w.id,
			"vault_id":  w.vaultID,
			"path":      event.Path,
			"error":     err,
		}).Error("DB update failed after retries, sending to DLQ")

		atomic.AddInt64(&w.failedCount, 1)

		select {
		case w.dlq <- event:
			logger.WithField("path", event.Path).Debug("Event sent to DLQ")
		default:
			logger.WithField("path", event.Path).Error("DLQ full, event permanently lost")
		}
		return
	}

	// Step 2: Update Explorer cache (synchronous)
	w.explorerService.InvalidateCacheSync(event)

	// Step 3: Update Index (best effort, synchronous)
	switch event.EventType {
	case syncpkg.FileCreated, syncpkg.FileModified:
		if err := w.indexService.ReIndexSync(event.Path); err != nil {
			logger.WithError(err).WithFields(map[string]interface{}{
				"worker_id": w.id,
				"path":      event.Path,
			}).Warn("Failed to update index")
		}
	case syncpkg.FileDeleted:
		if err := w.indexService.DeleteFromIndexSync(event.Path); err != nil {
			logger.WithError(err).WithFields(map[string]interface{}{
				"worker_id": w.id,
				"path":      event.Path,
			}).Warn("Failed to delete from index")
		}
	}

	// Step 4: Queue for SSE batching
	w.queueSSEEvent(event)

	atomic.AddInt64(&w.processedCount, 1)
}

// updateDBWithRetry attempts to update the database with retry logic
func (w *Worker) updateDBWithRetry(event syncpkg.FileChangeEvent) error {
	var lastErr error

	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		// Perform the DB update based on event type
		err := w.updateDatabase(event)
		if err == nil {
			if attempt > 0 {
				atomic.AddInt64(&w.retriedCount, 1)
				logger.WithFields(map[string]interface{}{
					"worker_id": w.id,
					"path":      event.Path,
					"attempt":   attempt + 1,
				}).Info("DB update succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Don't sleep on the last attempt
		if attempt < w.maxRetries {
			logger.WithFields(map[string]interface{}{
				"worker_id": w.id,
				"path":      event.Path,
				"attempt":   attempt + 1,
				"max":       w.maxRetries + 1,
				"error":     err,
			}).Warn("DB update failed, retrying after delay")

			select {
			case <-time.After(w.retryDelay):
				// Continue to next retry
			case <-w.ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", err)
			}
		}
	}

	return fmt.Errorf("DB update failed after %d attempts: %w", w.maxRetries+1, lastErr)
}

// updateDatabase performs the actual database update
func (w *Worker) updateDatabase(event syncpkg.FileChangeEvent) error {
	if w.dbService == nil {
		return fmt.Errorf("db service not initialized")
	}

	return performDatabaseUpdate(w.dbService, w.vaultPath, event)
}

// processDLQ processes failed events from the dead letter queue
func (w *Worker) processDLQ() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.dlqRetryDelay)
	defer ticker.Stop()

	var pendingEvents []syncpkg.FileChangeEvent

	for {
		select {
		case <-w.ctx.Done():
			logger.WithFields(map[string]interface{}{
				"worker_id":      w.id,
				"pending_in_dlq": len(pendingEvents),
			}).Info("DLQ processor stopped")
			return

		case event, ok := <-w.dlq:
			if !ok {
				return
			}
			pendingEvents = append(pendingEvents, event)
			logger.WithFields(map[string]interface{}{
				"worker_id":   w.id,
				"path":        event.Path,
				"dlq_pending": len(pendingEvents),
			}).Debug("Event added to DLQ")

		case <-ticker.C:
			if len(pendingEvents) == 0 {
				continue
			}

			logger.WithFields(map[string]interface{}{
				"worker_id": w.id,
				"count":     len(pendingEvents),
			}).Info("Processing DLQ events")

			// Try to reprocess events from DLQ
			stillFailing := make([]syncpkg.FileChangeEvent, 0)

			for _, event := range pendingEvents {
				err := w.updateDatabase(event)
				if err != nil {
					// Still failing, keep in DLQ for next retry
					stillFailing = append(stillFailing, event)
					logger.WithFields(map[string]interface{}{
						"worker_id": w.id,
						"path":      event.Path,
						"error":     err,
					}).Warn("DLQ event still failing")
				} else {
					// Success! Process remaining steps (synchronous)
					switch event.EventType {
					case syncpkg.FileCreated, syncpkg.FileModified:
						_ = w.indexService.ReIndexSync(event.Path)
					case syncpkg.FileDeleted:
						_ = w.indexService.DeleteFromIndexSync(event.Path)
					}
					w.explorerService.InvalidateCacheSync(event)
					w.queueSSEEvent(event)
					atomic.AddInt64(&w.processedCount, 1)

					logger.WithFields(map[string]interface{}{
						"worker_id": w.id,
						"path":      event.Path,
					}).Info("DLQ event recovered successfully")
				}
			}

			pendingEvents = stillFailing
		}
	}
}

// queueSSEEvent queues an SSE event for batching
// SECURITY: Converts absolute path to relative path and fetches ID from DB
func (w *Worker) queueSSEEvent(event syncpkg.FileChangeEvent) {
	var eventType sse.EventType
	switch event.EventType {
	case syncpkg.FileCreated:
		eventType = sse.EventFileCreated
	case syncpkg.FileModified:
		eventType = sse.EventFileModified
	case syncpkg.FileDeleted:
		eventType = sse.EventFileDeleted
	default:
		return
	}

	// Convert absolute path to relative path (NEVER send absolute paths to client!)
	relPath, err := filepath.Rel(w.vaultPath, event.Path)
	if err != nil {
		logger.WithError(err).WithFields(map[string]interface{}{
			"worker_id": w.id,
			"path":      event.Path,
		}).Warn("Failed to get relative path for SSE event")
		return
	}

	// Get file ID from DB (needed by UI to fetch content)
	var fileID string
	if w.dbService != nil {
		if entry, err := w.dbService.GetFileEntryByPath(relPath); err == nil && entry != nil {
			fileID = entry.ID
		}
	}

	sseEvent := sse.Event{
		Type:      eventType,
		VaultID:   w.vaultID,
		Path:      relPath, // Relative path only!
		FileID:    fileID,  // DB ID for fetching content
		Timestamp: event.Timestamp,
	}

	select {
	case w.sseChannel <- sseEvent:
		// Event queued for SSE batching
	default:
		logger.WithFields(map[string]interface{}{
			"worker_id": w.id,
			"path":      relPath,
		}).Warn("SSE channel full, dropping SSE event")
	}
}

// GetQueueDepth returns the current queue depth
func (w *Worker) GetQueueDepth() int {
	return len(w.queue)
}

// GetDLQDepth returns the current DLQ depth
func (w *Worker) GetDLQDepth() int {
	return len(w.dlq)
}

// GetMetrics returns worker metrics
func (w *Worker) GetMetrics() WorkerMetrics {
	return WorkerMetrics{
		WorkerID:       w.id,
		QueueDepth:     w.GetQueueDepth(),
		DLQDepth:       w.GetDLQDepth(),
		ProcessedCount: atomic.LoadInt64(&w.processedCount),
		FailedCount:    atomic.LoadInt64(&w.failedCount),
		RetriedCount:   atomic.LoadInt64(&w.retriedCount),
	}
}

// WorkerMetrics represents worker performance metrics
type WorkerMetrics struct {
	WorkerID       int
	QueueDepth     int
	DLQDepth       int
	ProcessedCount int64
	FailedCount    int64
	RetriedCount   int64
}
