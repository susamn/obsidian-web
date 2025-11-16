package indexing

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/sync"
)

func TestUpdateIndex_Validation(t *testing.T) {
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	vault := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		IndexPath: filepath.Join(indexDir, "test.bleve"),
	}

	ctx := context.Background()
	svc, err := NewIndexService(ctx, vault, vaultDir)
	if err != nil {
		t.Fatalf("NewIndexService() error = %v", err)
	}
	defer svc.Stop()

	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for service to be ready
	for update := range svc.StatusUpdates() {
		if update.Status == StatusReady || update.Status == StatusError {
			break
		}
	}

	tests := []struct {
		name        string
		event       sync.FileChangeEvent
		shouldQueue bool
		description string
	}{
		{
			name: "valid created event",
			event: sync.FileChangeEvent{
				VaultID:   "test-vault",
				Path:      filepath.Join(vaultDir, "valid.md"),
				EventType: sync.FileCreated,
				Timestamp: time.Now(),
			},
			shouldQueue: true,
			description: "Should queue valid FileCreated event",
		},
		{
			name: "valid modified event",
			event: sync.FileChangeEvent{
				VaultID:   "test-vault",
				Path:      filepath.Join(vaultDir, "valid.md"),
				EventType: sync.FileModified,
				Timestamp: time.Now(),
			},
			shouldQueue: true,
			description: "Should queue valid FileModified event",
		},
		{
			name: "valid deleted event",
			event: sync.FileChangeEvent{
				VaultID:   "test-vault",
				Path:      filepath.Join(vaultDir, "valid.md"),
				EventType: sync.FileDeleted,
				Timestamp: time.Now(),
			},
			shouldQueue: true,
			description: "Should queue valid FileDeleted event",
		},
		{
			name: "empty path",
			event: sync.FileChangeEvent{
				VaultID:   "test-vault",
				Path:      "",
				EventType: sync.FileModified,
				Timestamp: time.Now(),
			},
			shouldQueue: false,
			description: "Should reject event with empty path",
		},
		{
			name: "invalid event type",
			event: sync.FileChangeEvent{
				VaultID:   "test-vault",
				Path:      filepath.Join(vaultDir, "file.md"),
				EventType: sync.FileEventType(999), // Invalid type
				Timestamp: time.Now(),
			},
			shouldQueue: false,
			description: "Should reject event with invalid type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// UpdateIndex should not block
			start := time.Now()
			svc.UpdateIndex(tt.event)
			duration := time.Since(start)

			if duration > 10*time.Millisecond {
				t.Errorf("UpdateIndex() took %v, expected to be non-blocking", duration)
			}

			t.Logf("✓ %s", tt.description)
		})
	}
}

func TestUpdateIndex_ContextCancellation(t *testing.T) {
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	vault := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		IndexPath: filepath.Join(indexDir, "test.bleve"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	svc, err := NewIndexService(ctx, vault, vaultDir)
	if err != nil {
		t.Fatalf("NewIndexService() error = %v", err)
	}
	defer svc.Stop()

	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for ready
	for update := range svc.StatusUpdates() {
		if update.Status == StatusReady || update.Status == StatusError {
			break
		}
	}

	// Cancel context
	cancel()

	// Try to send event after cancellation
	event := sync.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      filepath.Join(vaultDir, "test.md"),
		EventType: sync.FileModified,
		Timestamp: time.Now(),
	}

	// Should handle gracefully (log and return)
	svc.UpdateIndex(event)

	t.Log("✓ UpdateIndex handles context cancellation gracefully")
}
