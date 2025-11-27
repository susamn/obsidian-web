package vault

// EventDispatcher is no longer needed for routing, kept only for metrics aggregation
// Workers now consume directly from sync channel
type EventDispatcher struct {
	workers []*Worker
}

// NewEventDispatcher creates a metrics aggregator for workers
func NewEventDispatcher(workers []*Worker) *EventDispatcher {
	return &EventDispatcher{
		workers: workers,
	}
}

// GetMetrics returns aggregated metrics from all workers
func (d *EventDispatcher) GetMetrics() DispatcherMetrics {
	numWorkers := len(d.workers)
	metrics := DispatcherMetrics{
		NumWorkers:     numWorkers,
		WorkerMetrics:  make([]WorkerMetrics, numWorkers),
		TotalProcessed: 0,
		TotalFailed:    0,
		TotalRetried:   0,
		TotalDLQ:       0,
	}

	for i, worker := range d.workers {
		wm := worker.GetMetrics()
		metrics.WorkerMetrics[i] = wm
		metrics.TotalProcessed += wm.ProcessedCount
		metrics.TotalFailed += wm.FailedCount
		metrics.TotalRetried += wm.RetriedCount
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
	TotalDLQ       int
}
