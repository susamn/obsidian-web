package vault

import (
	"context"
	"sync"
	"testing"
	"time"

	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// TestWorker_Creation tests that a worker can be created successfully
func TestWorker_Creation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	var wg sync.WaitGroup

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		nil, // DB service not required for creation test
		nil, // Index service not required for creation test
		nil, // Explorer service not required for creation test
	)

	if worker == nil {
		t.Fatal("Expected worker to be created, got nil")
	}

	if worker.id != 0 {
		t.Errorf("Expected worker ID 0, got %d", worker.id)
	}

	if worker.vaultID != "test-vault" {
		t.Errorf("Expected vault ID 'test-vault', got %s", worker.vaultID)
	}
}

// TestWorker_QueueDepth tests the queue depth calculation
func TestWorker_QueueDepth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	var wg sync.WaitGroup

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		nil,
		nil,
		nil,
	)

	// Initially queue should be empty
	if depth := worker.GetQueueDepth(); depth != 0 {
		t.Errorf("Expected initial queue depth of 0, got %d", depth)
	}

	// Add events to queue
	for i := 0; i < 5; i++ {
		worker.queue <- syncpkg.FileChangeEvent{
			Path:      "test.md",
			EventType: syncpkg.FileCreated,
			Timestamp: time.Now(),
		}
	}

	if depth := worker.GetQueueDepth(); depth != 5 {
		t.Errorf("Expected queue depth of 5, got %d", depth)
	}
}

// TestWorker_Metrics tests the metrics retrieval
func TestWorker_Metrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	var wg sync.WaitGroup

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		nil,
		nil,
		nil,
	)

	metrics := worker.GetMetrics()

	if metrics.WorkerID != 0 {
		t.Errorf("Expected worker ID 0, got %d", metrics.WorkerID)
	}

	if metrics.QueueDepth != 0 {
		t.Errorf("Expected initial queue depth 0, got %d", metrics.QueueDepth)
	}

	if metrics.ProcessedCount != 0 {
		t.Errorf("Expected initial processed count 0, got %d", metrics.ProcessedCount)
	}
}

// TestWorker_StartStop tests starting and stopping a worker
func TestWorker_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	var wg sync.WaitGroup

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		nil,
		nil,
		nil,
	)

	// Start worker
	worker.Start()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Stop by canceling context
	cancel()

	// Wait for cleanup with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("Worker did not stop within timeout")
	}
}
