package vault

import (
	"context"
	"hash/fnv"
	"sync"

	"github.com/susamn/obsidian-web/internal/logger"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// EventDispatcher routes file events to workers using hash-based routing
type EventDispatcher struct {
	ctx        context.Context
	vaultID    string
	workers    []*Worker
	numWorkers int
	wg         *sync.WaitGroup
}

// NewEventDispatcher creates a new event dispatcher
func NewEventDispatcher(ctx context.Context, vaultID string, workers []*Worker) *EventDispatcher {
	return &EventDispatcher{
		ctx:        ctx,
		vaultID:    vaultID,
		workers:    workers,
		numWorkers: len(workers),
		wg:         &sync.WaitGroup{},
	}
}

// Start begins routing events from the sync service to workers
func (d *EventDispatcher) Start(eventChan <-chan syncpkg.FileChangeEvent) {
	d.wg.Add(1)
	go d.route(eventChan)

	logger.WithFields(map[string]interface{}{
		"vault_id":    d.vaultID,
		"num_workers": d.numWorkers,
	}).Info("Event dispatcher started")
}

// route is the main event routing loop
func (d *EventDispatcher) route(eventChan <-chan syncpkg.FileChangeEvent) {
	defer d.wg.Done()

	for {
		select {
		case <-d.ctx.Done():
			logger.WithField("vault_id", d.vaultID).Info("Event dispatcher stopped")
			return

		case event, ok := <-eventChan:
			if !ok {
				logger.WithField("vault_id", d.vaultID).Info("Event channel closed")
				return
			}

			// Route event to worker based on path hash
			workerID := d.routeEvent(event)

			// Send to worker queue (non-blocking to prevent dispatcher deadlock)
			select {
			case d.workers[workerID].queue <- event:
				// Event successfully queued
			case <-d.ctx.Done():
				return
			default:
				// Worker queue full - this should be rare with 1000 capacity
				logger.WithFields(map[string]interface{}{
					"vault_id":  d.vaultID,
					"worker_id": workerID,
					"path":      event.Path,
				}).Warn("Worker queue full, dropping event")
			}
		}
	}
}

// routeEvent determines which worker should handle the event
// Uses hash-based routing to ensure same file always goes to same worker
func (d *EventDispatcher) routeEvent(event syncpkg.FileChangeEvent) int {
	h := fnv.New32a()
	h.Write([]byte(event.Path))
	return int(h.Sum32()) % d.numWorkers
}

// Wait waits for the dispatcher to finish
func (d *EventDispatcher) Wait() {
	d.wg.Wait()
}

// GetMetrics returns aggregated metrics from all workers
func (d *EventDispatcher) GetMetrics() DispatcherMetrics {
	metrics := DispatcherMetrics{
		NumWorkers:     d.numWorkers,
		WorkerMetrics:  make([]WorkerMetrics, d.numWorkers),
		TotalProcessed: 0,
		TotalFailed:    0,
		TotalRetried:   0,
		TotalQueued:    0,
		TotalDLQ:       0,
	}

	for i, worker := range d.workers {
		wm := worker.GetMetrics()
		metrics.WorkerMetrics[i] = wm
		metrics.TotalProcessed += wm.ProcessedCount
		metrics.TotalFailed += wm.FailedCount
		metrics.TotalRetried += wm.RetriedCount
		metrics.TotalQueued += wm.QueueDepth
		metrics.TotalDLQ += wm.DLQDepth
	}

	return metrics
}

// DispatcherMetrics represents aggregated dispatcher metrics
type DispatcherMetrics struct {
	NumWorkers     int
	WorkerMetrics  []WorkerMetrics
	TotalProcessed int64
	TotalFailed    int64
	TotalRetried   int64
	TotalQueued    int
	TotalDLQ       int
}
