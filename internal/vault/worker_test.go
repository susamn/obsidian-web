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

// TestWorker_DLQDepth tests the DLQ depth calculation
func TestWorker_DLQDepth(t *testing.T) {
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

	// Initially DLQ should be empty
	if depth := worker.GetDLQDepth(); depth != 0 {
		t.Errorf("Expected initial DLQ depth of 0, got %d", depth)
	}

	// Add events to DLQ
	for i := 0; i < 5; i++ {
		worker.dlq <- syncpkg.FileChangeEvent{
			Path:      "test.md",
			EventType: syncpkg.FileCreated,
			Timestamp: time.Now(),
		}
	}

	if depth := worker.GetDLQDepth(); depth != 5 {
		t.Errorf("Expected DLQ depth of 5, got %d", depth)
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

	if metrics.DLQDepth != 0 {
		t.Errorf("Expected initial DLQ depth 0, got %d", metrics.DLQDepth)
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

	// Create a sync channel for the worker
	syncEvents := make(chan syncpkg.FileChangeEvent, 10)

	// Start worker
	worker.Start(syncEvents)

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
