package recon

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// TestReconciliationService_Creation tests creation of reconciliation service
func TestReconciliationService_Creation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	recon := NewReconciliationService("test-vault", ctx, &wg)

	if recon == nil {
		t.Fatal("Expected reconciliation service to be created")
	}

	if recon.vaultID != "test-vault" {
		t.Errorf("Expected vault ID 'test-vault', got %s", recon.vaultID)
	}

	if recon.retryInterval != 5*time.Second {
		t.Errorf("Expected retry interval 5s, got %v", recon.retryInterval)
	}
}

// TestReconciliationService_SendToDLQ tests sending events to DLQ
func TestReconciliationService_SendToDLQ(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	recon := NewReconciliationService("test-vault", ctx, &wg)

	// Initially DLQ should be empty
	if depth := recon.GetDLQDepth(); depth != 0 {
		t.Errorf("Expected initial DLQ depth of 0, got %d", depth)
	}

	// Send events to DLQ
	for i := 0; i < 5; i++ {
		recon.SendToDLQ(syncpkg.FileChangeEvent{
			Path:      "test.md",
			EventType: syncpkg.FileCreated,
			Timestamp: time.Now(),
		})
	}

	if depth := recon.GetDLQDepth(); depth != 5 {
		t.Errorf("Expected DLQ depth of 5, got %d", depth)
	}

	metrics := recon.GetMetrics()
	if metrics.DLQCount != 5 {
		t.Errorf("Expected DLQ count of 5, got %d", metrics.DLQCount)
	}
}

// TestReconciliationService_DLQFull tests behavior when DLQ is full
func TestReconciliationService_DLQFull(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	recon := NewReconciliationService("test-vault", ctx, &wg)

	// Fill the DLQ (1000 capacity)
	for i := 0; i < 1000; i++ {
		recon.SendToDLQ(syncpkg.FileChangeEvent{
			Path:      "test.md",
			EventType: syncpkg.FileCreated,
			Timestamp: time.Now(),
		})
	}

	metrics := recon.GetMetrics()
	if metrics.DLQCount != 1000 {
		t.Errorf("Expected DLQ count of 1000, got %d", metrics.DLQCount)
	}

	if metrics.DroppedCount != 0 {
		t.Errorf("Expected dropped count of 0, got %d", metrics.DroppedCount)
	}

	// Try to send one more - should be dropped
	recon.SendToDLQ(syncpkg.FileChangeEvent{
		Path:      "overflow.md",
		EventType: syncpkg.FileCreated,
		Timestamp: time.Now(),
	})

	metrics = recon.GetMetrics()
	if metrics.DroppedCount != 1 {
		t.Errorf("Expected dropped count of 1, got %d", metrics.DroppedCount)
	}

	if recon.GetDLQDepth() != 1000 {
		t.Errorf("Expected DLQ depth to remain at 1000, got %d", recon.GetDLQDepth())
	}
}

// TestReconciliationService_RetryMechanism tests the retry mechanism with mock sync service
func TestReconciliationService_RetryMechanism(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Create a mock sync service
	syncSvc, err := syncpkg.NewSyncService(ctx, "test-vault", &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	recon := NewReconciliationService("test-vault", ctx, &wg)
	recon.SetSyncService(syncSvc)
	recon.retryInterval = 100 * time.Millisecond // Speed up for testing

	// Start the service
	recon.Start()

	// Send events to DLQ
	for i := 0; i < 10; i++ {
		recon.SendToDLQ(syncpkg.FileChangeEvent{
			Path:      "test.md",
			EventType: syncpkg.FileCreated,
			Timestamp: time.Now(),
		})
	}

	// Wait for retry interval
	time.Sleep(200 * time.Millisecond)

	metrics := recon.GetMetrics()
	if metrics.RetriedCount != 10 {
		t.Errorf("Expected retried count of 10, got %d", metrics.RetriedCount)
	}

	// Stop the service
	cancel()
	wg.Wait()
}

// TestReconciliationService_Metrics tests metrics retrieval
func TestReconciliationService_Metrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	recon := NewReconciliationService("test-vault", ctx, &wg)

	metrics := recon.GetMetrics()
	if metrics.DLQDepth != 0 {
		t.Errorf("Expected initial DLQ depth of 0, got %d", metrics.DLQDepth)
	}
	if metrics.DLQCount != 0 {
		t.Errorf("Expected initial DLQ count of 0, got %d", metrics.DLQCount)
	}
	if metrics.RetriedCount != 0 {
		t.Errorf("Expected initial retried count of 0, got %d", metrics.RetriedCount)
	}
	if metrics.DroppedCount != 0 {
		t.Errorf("Expected initial dropped count of 0, got %d", metrics.DroppedCount)
	}
}
