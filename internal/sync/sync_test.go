package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
)

func TestNewSyncService(t *testing.T) {
	tests := []struct {
		name      string
		vaultID   string
		storage   *config.StorageConfig
		wantErr   bool
		errString string
	}{
		{
			name:    "valid local storage",
			vaultID: "test-vault",
			storage: &config.StorageConfig{
				Type: "local",
				Local: &config.LocalStorageConfig{
					Path: t.TempDir(),
				},
			},
			wantErr: false,
		},
		{
			name:      "nil storage config",
			vaultID:   "test-vault",
			storage:   nil,
			wantErr:   true,
			errString: "storage config cannot be nil",
		},
		{
			name:    "local storage with invalid path",
			vaultID: "test-vault",
			storage: &config.StorageConfig{
				Type: "local",
				Local: &config.LocalStorageConfig{
					Path: "/nonexistent/path/to/vault",
				},
			},
			wantErr:   true,
			errString: "failed to create local sync",
		},
		{
			name:    "s3 storage",
			vaultID: "s3-vault",
			storage: &config.StorageConfig{
				Type: "s3",
				S3: &config.S3StorageConfig{
					Bucket: "test-bucket",
					Region: "us-east-1",
				},
			},
			wantErr: false,
		},
		{
			name:    "minio storage",
			vaultID: "minio-vault",
			storage: &config.StorageConfig{
				Type: "minio",
				MinIO: &config.MinIOStorageConfig{
					Endpoint: "localhost:9000",
					Bucket:   "test-bucket",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			svc, err := NewSyncService(ctx, tt.vaultID, tt.storage)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewSyncService() expected error, got nil")
				}
				if tt.errString != "" && err != nil {
					// Check if error contains expected string
					if !contains(err.Error(), tt.errString) {
						t.Errorf("NewSyncService() error = %v, want error containing %v", err, tt.errString)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("NewSyncService() unexpected error = %v", err)
				return
			}

			if svc == nil {
				t.Error("NewSyncService() returned nil service")
				return
			}

			if svc.VaultID() != tt.vaultID {
				t.Errorf("SyncService.VaultID() = %v, want %v", svc.VaultID(), tt.vaultID)
			}

			// Cleanup
			_ = svc.Stop()
		})
	}
}

func TestSyncService_LocalFileChanges(t *testing.T) {
	// Create temporary directory for test vault
	tmpDir := t.TempDir()

	// Create sync service
	ctx := context.Background()
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}
	defer svc.Stop()

	// Start the service (non-blocking)
	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	// Give the watcher time to initialize
	time.Sleep(100 * time.Millisecond)

	// Test file creation
	t.Run("file creation", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.md")
		if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Wait for event
		select {
		case event := <-svc.Events():
			if event.EventType != FileCreated {
				t.Errorf("Expected FileCreated event, got %v", event.EventType)
			}
			if event.VaultID != "test-vault" {
				t.Errorf("Expected vault ID 'test-vault', got %v", event.VaultID)
			}
			if !contains(event.Path, "test.md") {
				t.Errorf("Expected path to contain 'test.md', got %v", event.Path)
			}
		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for file creation event")
		}
	})

	// Test file modification
	t.Run("file modification", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.md")
		if err := os.WriteFile(testFile, []byte("# Modified"), 0644); err != nil {
			t.Fatalf("Failed to modify test file: %v", err)
		}

		// Wait for event
		select {
		case event := <-svc.Events():
			if event.EventType != FileModified {
				t.Errorf("Expected FileModified event, got %v", event.EventType)
			}
		case <-time.After(2 * time.Second):
			t.Error("Timeout waiting for file modification event")
		}
	})

	// Test file deletion
	t.Run("file deletion", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.md")
		if err := os.Remove(testFile); err != nil {
			t.Fatalf("Failed to delete test file: %v", err)
		}

		// Wait for event (may receive multiple events, look for delete)
		timeout := time.After(2 * time.Second)
		foundDelete := false
		for !foundDelete {
			select {
			case event := <-svc.Events():
				if event.EventType == FileDeleted {
					foundDelete = true
				}
			case <-timeout:
				t.Error("Timeout waiting for file deletion event")
				return
			}
		}
	})

	// Test all file types are now processed (not just markdown)
	t.Run("process all file types", func(t *testing.T) {
		// Drain any pending events from previous tests
		drainEvents(svc.Events(), 100*time.Millisecond)

		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Should receive event for .txt file (all file types are now tracked)
		timeout := time.After(500 * time.Millisecond)
		select {
		case event := <-svc.Events():
			// Check if it's for the .txt file
			if filepath.Ext(event.Path) != ".txt" {
				t.Errorf("Expected event for .txt file, got event for: %s", event.Path)
			}
			if event.EventType != FileCreated {
				t.Errorf("Expected FileCreated event, got %v", event.EventType)
			}
		case <-timeout:
			t.Error("Expected to receive event for non-markdown file, but got none")
		}

		// Cleanup
		_ = os.Remove(testFile)
	})
}

func TestSyncService_NonBlocking(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}
	defer svc.Stop()

	// Start should return immediately (non-blocking)
	start := time.Now()
	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Errorf("Start() took too long (%v), expected to be non-blocking", duration)
	}
}

func TestSyncService_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Stop should complete gracefully
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}

	// Events channel should be closed
	select {
	case _, ok := <-svc.Events():
		if ok {
			t.Error("Events channel should be closed after Stop()")
		}
	case <-time.After(1 * time.Second):
		t.Error("Events channel not closed after Stop()")
	}
}

func TestSyncService_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}
	defer svc.Stop()

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	cancel()

	// Events channel should close
	select {
	case _, ok := <-svc.Events():
		if ok {
			t.Error("Events channel should be closed after context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Error("Events channel not closed after context cancellation")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper to drain events from channel
func drainEvents(events <-chan FileChangeEvent, timeout time.Duration) {
	deadline := time.After(timeout)
	for {
		select {
		case <-events:
			// Drain event
		case <-deadline:
			return
		default:
			return
		}
	}
}

// ====================
// Integration Tests
// ====================

func TestSyncService_IntegrationTest(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create sync service
	ctx := context.Background()
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	// Start service
	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	// Give watcher time to initialize
	time.Sleep(200 * time.Millisecond)

	// Create a markdown file
	testFile := filepath.Join(tmpDir, "test-note.md")
	testContent := []byte("# Test Note\n\nThis is a test markdown file.")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for and verify the event
	select {
	case event := <-svc.Events():
		// Verify event details
		if event.VaultID != "test-vault" {
			t.Errorf("Expected vault ID 'test-vault', got '%s'", event.VaultID)
		}
		if event.EventType != FileCreated {
			t.Errorf("Expected FileCreated event, got %s", event.EventType)
		}
		if !contains(event.Path, "test-note.md") {
			t.Errorf("Expected path to contain 'test-note.md', got '%s'", event.Path)
		}
		if event.Timestamp.IsZero() {
			t.Error("Expected non-zero timestamp")
		}

		t.Logf("✓ Received event: %s - %s at %s", event.EventType, event.Path, event.Timestamp)

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout: Did not receive file creation event")
	}

	// Clean up: Stop the service
	if err := svc.Stop(); err != nil {
		t.Errorf("Failed to stop service: %v", err)
	}

	// t.TempDir() automatically cleans up the directory after test completes
	t.Log("✓ Test completed, temporary directory will be cleaned up automatically")
}

// ====================
// Deadlock Tests
// ====================

func TestSyncService_NoDeadlock_FullEventBuffer(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}
	defer svc.Stop()

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create more files than the buffer size (100) WITHOUT consuming events
	// This tests that the service doesn't deadlock when buffer is full
	for i := 0; i < 150; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.md", i))
		if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Wait a bit for events to queue up
	time.Sleep(500 * time.Millisecond)

	// Stop should not deadlock even with full buffer
	done := make(chan error, 1)
	go func() {
		done <- svc.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Stop() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock detected: Stop() did not complete within timeout")
	}
}

func TestSyncService_NoDeadlock_StopWithoutConsumer(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create some files
	for i := 0; i < 10; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.md", i))
		_ = os.WriteFile(testFile, []byte("# Test"), 0644)
	}

	time.Sleep(200 * time.Millisecond)

	// Stop WITHOUT consuming any events
	// This tests that Stop() doesn't deadlock waiting for events to be consumed
	done := make(chan error, 1)
	go func() {
		done <- svc.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Stop() returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Deadlock detected: Stop() without consumer did not complete")
	}
}

func TestSyncService_NoDeadlock_MultipleStopCalls(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Call Stop() multiple times concurrently
	// This should not cause deadlock or panic
	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		go func() {
			_ = svc.Stop()
			done <- struct{}{}
		}()
	}

	// Wait for all Stop() calls to complete
	timeout := time.After(3 * time.Second)
	for i := 0; i < 5; i++ {
		select {
		case <-done:
			// Stop call completed
		case <-timeout:
			t.Fatal("Deadlock detected: Multiple Stop() calls did not complete")
		}
	}
}

func TestSyncService_NoDeadlock_ConcurrentStopAndEventProcessing(t *testing.T) {
	tmpDir := t.TempDir()

	ctx := context.Background()
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Start consuming events in background
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for range svc.Events() {
			// Consume events
			time.Sleep(10 * time.Millisecond) // Slow consumer
		}
	}()

	// Generate events while consuming
	go func() {
		for i := 0; i < 20; i++ {
			testFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.md", i))
			_ = os.WriteFile(testFile, []byte("# Test"), 0644)
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Wait a bit for some events to be generated
	time.Sleep(200 * time.Millisecond)

	// Call Stop() while events are being consumed
	stopDone := make(chan error, 1)
	go func() {
		stopDone <- svc.Stop()
	}()

	// Verify both complete without deadlock
	timeout := time.After(5 * time.Second)

	select {
	case err := <-stopDone:
		if err != nil {
			t.Errorf("Stop() returned error: %v", err)
		}
	case <-timeout:
		t.Fatal("Deadlock detected: Stop() with concurrent event processing did not complete")
	}

	select {
	case <-consumerDone:
		// Consumer finished
	case <-timeout:
		t.Fatal("Deadlock detected: Event consumer did not finish after Stop()")
	}
}

func TestSyncService_NoDeadlock_RapidStartStop(t *testing.T) {
	tmpDir := t.TempDir()

	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	// Rapid start/stop cycles should not cause deadlock
	for i := 0; i < 10; i++ {
		ctx := context.Background()
		svc, err := NewSyncService(ctx, "test-vault", storage)
		if err != nil {
			t.Fatalf("Failed to create sync service: %v", err)
		}

		if err := svc.Start(); err != nil {
			t.Fatalf("Failed to start sync service: %v", err)
		}

		// Give a tiny bit of time for Start() goroutine to initialize
		time.Sleep(10 * time.Millisecond)

		// Immediately stop (very short-lived service)
		done := make(chan error, 1)
		go func() {
			done <- svc.Stop()
		}()

		select {
		case err := <-done:
			// In rapid start/stop, we may get errors due to race conditions
			// The important thing is that it doesn't deadlock
			if err != nil && !contains(err.Error(), "watcher already closed") {
				t.Errorf("Iteration %d: Stop() returned unexpected error: %v", i, err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("Iteration %d: Deadlock detected in rapid start/stop", i)
		}
	}
}

func TestSyncService_NoDeadlock_ContextCancelDuringFileCreation(t *testing.T) {
	tmpDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	storage := &config.StorageConfig{
		Type: "local",
		Local: &config.LocalStorageConfig{
			Path: tmpDir,
		},
	}

	svc, err := NewSyncService(ctx, "test-vault", storage)
	if err != nil {
		t.Fatalf("Failed to create sync service: %v", err)
	}
	defer svc.Stop()

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start sync service: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Start creating files
	go func() {
		for i := 0; i < 50; i++ {
			testFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.md", i))
			_ = os.WriteFile(testFile, []byte("# Test"), 0644)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Cancel context while files are being created
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Verify Stop() completes without deadlock
	done := make(chan error, 1)
	go func() {
		done <- svc.Stop()
	}()

	select {
	case err := <-done:
		if err != nil && !contains(err.Error(), "context canceled") {
			t.Errorf("Stop() returned unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Deadlock detected: Stop() after context cancel did not complete")
	}
}
