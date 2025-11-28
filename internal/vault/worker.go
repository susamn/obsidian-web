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
	"github.com/susamn/obsidian-web/internal/recon"
	"github.com/susamn/obsidian-web/internal/sse"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// Worker processes file events directly from sync channel with DB-first approach
// No internal queuing - workers consume events directly from shared sync channel
type Worker struct {
	id        int
	vaultID   string
	vaultPath string
	ctx       context.Context
	wg        *sync.WaitGroup

	// Services
	dbService       *db.DBService
	indexService    *indexing.IndexService
	explorerService *explorer.ExplorerService
	reconService    *recon.ReconciliationService
	sseManager      *sse.Manager

	// Metrics
	processedCount int64
	failedCount    int64

	// Configuration
	maxRetries int
	retryDelay time.Duration
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
	reconService *recon.ReconciliationService,
) *Worker {
	return &Worker{
		id:              id,
		vaultID:         vaultID,
		vaultPath:       vaultPath,
		ctx:             ctx,
		wg:              wg,
		dbService:       dbService,
		indexService:    indexService,
		explorerService: explorerService,
		reconService:    reconService,
		maxRetries:      2,
		retryDelay:      2 * time.Second,
	}
}

// Start starts the worker processing loop consuming from shared sync channel
func (w *Worker) Start(syncEvents <-chan syncpkg.FileChangeEvent) {
	w.wg.Add(1)
	go w.run(syncEvents)

	logger.WithFields(map[string]interface{}{
		"worker_id": w.id,
		"vault_id":  w.vaultID,
	}).Info("Worker started")
}

// run is the main event processing loop consuming directly from sync channel
// Multiple workers share the same sync channel for load balancing
func (w *Worker) run(syncEvents <-chan syncpkg.FileChangeEvent) {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			logger.WithFields(map[string]interface{}{
				"worker_id": w.id,
				"vault_id":  w.vaultID,
				"processed": atomic.LoadInt64(&w.processedCount),
				"failed":    atomic.LoadInt64(&w.failedCount),
			}).Info("Worker stopped")
			return

		case event, ok := <-syncEvents:
			if !ok {
				logger.WithField("worker_id", w.id).Info("Sync channel closed")
				return
			}
			w.processEvent(event)
		}
	}
}

// processEvent handles a single file event
func (w *Worker) processEvent(event syncpkg.FileChangeEvent) {
	// Step 1: Update DB with retry logic and get file ID
	fileID, err := w.updateDBWithRetry(event)
	if err != nil {
		// DB update failed after retries, send to DLQ
		logger.WithFields(map[string]interface{}{
			"worker_id": w.id,
			"vault_id":  w.vaultID,
			"path":      event.Path,
			"error":     err,
		}).Error("DB update failed after retries, sending to DLQ")

		atomic.AddInt64(&w.failedCount, 1)

		// Send to reconciliation service DLQ
		w.reconService.SendToDLQ(event)
		return
	}

	// Step 2: Update Explorer cache (synchronous)
	w.explorerService.InvalidateCacheSync(event)

	// Step 3: Update Index (best effort, synchronous) with file ID
	// Check if index service is ready first (avoids race condition during startup)
	indexStatus := w.indexService.GetStatus()
	if indexStatus != indexing.StatusReady {
		logger.WithFields(map[string]interface{}{
			"worker_id":    w.id,
			"path":         event.Path,
			"index_status": indexStatus.String(),
		}).Debug("Skipping index update - service not ready yet")
	} else {
		switch event.EventType {
		case syncpkg.FileCreated, syncpkg.FileModified:
			if err := w.indexService.ReIndexSync(event.Path, fileID); err != nil {
				logger.WithError(err).WithFields(map[string]interface{}{
					"worker_id": w.id,
					"path":      event.Path,
					"file_id":   fileID,
				}).Warn("Failed to update index")
			}
		case syncpkg.FileDeleted:
			if err := w.indexService.DeleteFromIndexSync(event.Path, fileID); err != nil {
				logger.WithError(err).WithFields(map[string]interface{}{
					"worker_id": w.id,
					"path":      event.Path,
					"file_id":   fileID,
				}).Warn("Failed to delete from index")
			}
		}
	}

	// Step 4: Queue for SSE batching
	w.queueSSEEvent(event)

	atomic.AddInt64(&w.processedCount, 1)
}

// updateDBWithRetry attempts to update the database with retry logic
// Returns the file ID and error
func (w *Worker) updateDBWithRetry(event syncpkg.FileChangeEvent) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= w.maxRetries; attempt++ {
		// Perform the DB update based on event type
		fileID, err := w.updateDatabase(event)
		if err == nil {
			if attempt > 0 {
				logger.WithFields(map[string]interface{}{
					"worker_id": w.id,
					"path":      event.Path,
					"file_id":   fileID,
					"attempt":   attempt + 1,
				}).Info("DB update succeeded after retry")
			}
			return fileID, nil
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
				return "", fmt.Errorf("context cancelled during retry: %w", err)
			}
		}
	}

	return "", fmt.Errorf("DB update failed after %d attempts: %w", w.maxRetries+1, lastErr)
}

// updateDatabase performs the actual database update
// Returns the file ID and error
func (w *Worker) updateDatabase(event syncpkg.FileChangeEvent) (string, error) {
	if w.dbService == nil {
		return "", fmt.Errorf("db service not initialized")
	}

	return w.dbService.PerformDatabaseUpdate(w.vaultPath, event)
}

// queueSSEEvent queues an SSE event
// SECURITY: Converts absolute path to relative path and fetches ID from DB
func (w *Worker) queueSSEEvent(event syncpkg.FileChangeEvent) {
	if w.sseManager == nil {
		return
	}

	var action sse.ActionType
	switch event.EventType {
	case syncpkg.FileCreated:
		action = sse.ActionCreate
	case syncpkg.FileModified:
		action = sse.ActionCreate // Modified is treated as create for frontend
	case syncpkg.FileDeleted:
		action = sse.ActionDelete
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

	// Queue the file change in SSE manager
	w.sseManager.QueueFileChange(w.vaultID, fileID, relPath, action)
}

// GetMetrics returns worker metrics
func (w *Worker) GetMetrics() WorkerMetrics {
	return WorkerMetrics{
		WorkerID:       w.id,
		ProcessedCount: atomic.LoadInt64(&w.processedCount),
		FailedCount:    atomic.LoadInt64(&w.failedCount),
	}
}

// WorkerMetrics represents worker performance metrics
type WorkerMetrics struct {
	WorkerID       int
	ProcessedCount int64
	FailedCount    int64
}
