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

	recon := NewReconciliationService("test-vault", ctx, &wg)

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		nil,   // DB service not required for creation test
		nil,   // Index service not required for creation test
		nil,   // Explorer service not required for creation test
		recon, // Recon service
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

// TestWorker_ReconServiceIntegration tests worker integration with recon service
func TestWorker_ReconServiceIntegration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	var wg sync.WaitGroup

	recon := NewReconciliationService("test-vault", ctx, &wg)

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		nil,
		nil,
		nil,
		recon,
	)

	if worker.reconService == nil {
		t.Error("Expected worker to have reconciliation service")
	}

	// Send events to recon DLQ through worker's recon service
	for i := 0; i < 5; i++ {
		worker.reconService.SendToDLQ(syncpkg.FileChangeEvent{
			Path:      "test.md",
			EventType: syncpkg.FileCreated,
			Timestamp: time.Now(),
		})
	}

	if depth := recon.GetDLQDepth(); depth != 5 {
		t.Errorf("Expected DLQ depth of 5, got %d", depth)
	}
}

// TestWorker_Metrics tests the metrics retrieval
func TestWorker_Metrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	var wg sync.WaitGroup

	recon := NewReconciliationService("test-vault", ctx, &wg)

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		nil,
		nil,
		nil,
		recon,
	)

	metrics := worker.GetMetrics()

	if metrics.WorkerID != 0 {
		t.Errorf("Expected worker ID 0, got %d", metrics.WorkerID)
	}

	if metrics.ProcessedCount != 0 {
		t.Errorf("Expected initial processed count 0, got %d", metrics.ProcessedCount)
	}

	if metrics.FailedCount != 0 {
		t.Errorf("Expected initial failed count 0, got %d", metrics.FailedCount)
	}
}

// TestWorker_StartStop tests starting and stopping a worker
func TestWorker_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	var wg sync.WaitGroup
	syncEvents := make(chan syncpkg.FileChangeEvent, 10)

	recon := NewReconciliationService("test-vault", ctx, &wg)

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		nil,
		nil,
		nil,
		recon,
	)

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
