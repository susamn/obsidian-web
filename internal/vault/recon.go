package vault

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/susamn/obsidian-web/internal/logger"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// ReconciliationService handles failed events and retries them
// All workers share the same DLQ through this service
type ReconciliationService struct {
	vaultID        string
	dlq            chan syncpkg.FileChangeEvent
	syncServiceRef *syncpkg.SyncService // Reference to sync service to retry events
	ctx            context.Context
	wg             *sync.WaitGroup
	retryInterval  time.Duration

	// Metrics
	dlqCount     int64
	retriedCount int64
	droppedCount int64
}

// NewReconciliationService creates a new reconciliation service
func NewReconciliationService(
	vaultID string,
	ctx context.Context,
	wg *sync.WaitGroup,
) *ReconciliationService {
	return &ReconciliationService{
		vaultID:       vaultID,
		dlq:           make(chan syncpkg.FileChangeEvent, 1000),
		ctx:           ctx,
		wg:            wg,
		retryInterval: 5 * time.Second,
	}
}

// SetSyncService sets the sync service reference for retrying events
func (r *ReconciliationService) SetSyncService(syncService *syncpkg.SyncService) {
	r.syncServiceRef = syncService
}

// Start starts the reconciliation service
func (r *ReconciliationService) Start() {
	r.wg.Add(1)
	go r.processDLQ()

	logger.WithFields(map[string]interface{}{
		"vault_id":       r.vaultID,
		"retry_interval": r.retryInterval,
	}).Info("Reconciliation service started")
}

// SendToDLQ sends a failed event to the DLQ for retry
// Non-blocking - drops event if DLQ is full
func (r *ReconciliationService) SendToDLQ(event syncpkg.FileChangeEvent) {
	select {
	case r.dlq <- event:
		atomic.AddInt64(&r.dlqCount, 1)
		logger.WithFields(map[string]interface{}{
			"vault_id": r.vaultID,
			"path":     event.Path,
			"dlq_size": len(r.dlq),
		}).Debug("Event sent to DLQ")
	default:
		atomic.AddInt64(&r.droppedCount, 1)
		logger.WithFields(map[string]interface{}{
			"vault_id": r.vaultID,
			"path":     event.Path,
		}).Error("DLQ full, event permanently dropped")
	}
}

// processDLQ processes events from the DLQ and sends them back to sync channel
func (r *ReconciliationService) processDLQ() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.retryInterval)
	defer ticker.Stop()

	var pendingEvents []syncpkg.FileChangeEvent

	for {
		select {
		case <-r.ctx.Done():
			logger.WithFields(map[string]interface{}{
				"vault_id": r.vaultID,
				"pending":  len(pendingEvents),
				"retried":  atomic.LoadInt64(&r.retriedCount),
				"dropped":  atomic.LoadInt64(&r.droppedCount),
			}).Info("Reconciliation service stopped")
			return

		case event, ok := <-r.dlq:
			if !ok {
				return
			}
			pendingEvents = append(pendingEvents, event)
			logger.WithFields(map[string]interface{}{
				"vault_id": r.vaultID,
				"path":     event.Path,
				"pending":  len(pendingEvents),
			}).Debug("Event added to DLQ pending list")

		case <-ticker.C:
			if len(pendingEvents) == 0 {
				continue
			}

			logger.WithFields(map[string]interface{}{
				"vault_id": r.vaultID,
				"count":    len(pendingEvents),
			}).Info("Retrying DLQ events")

			// Send all pending events back to sync channel
			stillPending := make([]syncpkg.FileChangeEvent, 0)

			for _, event := range pendingEvents {
				if r.syncServiceRef != nil {
					// Try to inject event back into sync service
					if r.syncServiceRef.InjectEvent(event) {
						atomic.AddInt64(&r.retriedCount, 1)
						logger.WithFields(map[string]interface{}{
							"vault_id": r.vaultID,
							"path":     event.Path,
						}).Debug("Event reinjected to sync service from DLQ")
					} else {
						// Sync channel full, keep in DLQ for next retry
						stillPending = append(stillPending, event)
						logger.WithFields(map[string]interface{}{
							"vault_id": r.vaultID,
							"path":     event.Path,
						}).Warn("Sync channel full, event remains in DLQ")
					}
				} else {
					// No sync service reference, keep pending
					stillPending = append(stillPending, event)
				}
			}

			pendingEvents = stillPending
		}
	}
}

// GetDLQDepth returns the current DLQ depth
func (r *ReconciliationService) GetDLQDepth() int {
	return len(r.dlq)
}

// GetMetrics returns reconciliation service metrics
func (r *ReconciliationService) GetMetrics() ReconciliationMetrics {
	return ReconciliationMetrics{
		DLQDepth:     r.GetDLQDepth(),
		DLQCount:     atomic.LoadInt64(&r.dlqCount),
		RetriedCount: atomic.LoadInt64(&r.retriedCount),
		DroppedCount: atomic.LoadInt64(&r.droppedCount),
	}
}

// ReconciliationMetrics represents reconciliation service metrics
type ReconciliationMetrics struct {
	DLQDepth     int   // Current number of events in DLQ
	DLQCount     int64 // Total events sent to DLQ
	RetriedCount int64 // Total events retried
	DroppedCount int64 // Total events dropped (DLQ full)
}
