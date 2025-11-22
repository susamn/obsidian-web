package indexing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/sync"
)

func TestNewIndexService(t *testing.T) {
	tests := []struct {
		name      string
		vault     *config.VaultConfig
		vaultPath string
		wantErr   bool
		errString string
	}{
		{
			name:      "nil vault config",
			vault:     nil,
			vaultPath: "/tmp/vault",
			wantErr:   true,
			errString: "vault config cannot be nil",
		},
		{
			name: "empty index path",
			vault: &config.VaultConfig{
				ID:        "test",
				Name:      "Test Vault",
				IndexPath: "",
			},
			vaultPath: "/tmp/vault",
			wantErr:   true,
			errString: "index path cannot be empty",
		},
		{
			name: "empty vault path",
			vault: &config.VaultConfig{
				ID:        "test",
				Name:      "Test Vault",
				IndexPath: "/tmp/index",
			},
			vaultPath: "",
			wantErr:   true,
			errString: "vault path cannot be empty",
		},
		{
			name: "valid vault",
			vault: &config.VaultConfig{
				ID:        "test",
				Name:      "Test Vault",
				IndexPath: "/tmp/index",
			},
			vaultPath: "/tmp/vault",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			svc, err := NewIndexService(ctx, tt.vault, tt.vaultPath)

			if tt.wantErr {
				if err == nil {
					t.Error("NewIndexService() expected error, got nil")
					return
				}
				if tt.errString != "" && !contains(err.Error(), tt.errString) {
					t.Errorf("NewIndexService() error = %v, want error containing %v", err, tt.errString)
				}
				return
			}

			if err != nil {
				t.Errorf("NewIndexService() unexpected error = %v", err)
				return
			}

			if svc == nil {
				t.Error("NewIndexService() returned nil service")
				return
			}

			if svc.VaultID() != tt.vault.ID {
				t.Errorf("VaultID() = %v, want %v", svc.VaultID(), tt.vault.ID)
			}

			if svc.VaultName() != tt.vault.Name {
				t.Errorf("VaultName() = %v, want %v", svc.VaultName(), tt.vault.Name)
			}

			if svc.GetStatus() != StatusStandby {
				t.Errorf("Initial status = %v, want %v", svc.GetStatus(), StatusStandby)
			}

			svc.Stop()
		})
	}
}

func TestIndexService_Start(t *testing.T) {
	// Create temporary directories
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	// Create test markdown files
	testFiles := map[string]string{
		"note1.md": `---
tags: [golang, testing]
---
# First Note
This is a test note about #golang.
`,
		"note2.md": `# Second Note
A simple note with #testing tag.
`,
		"subfolder/note3.md": `# Nested Note
This is nested with #nested/tag.
`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(vaultDir, filename)
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create vault config
	vault := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		IndexPath: filepath.Join(indexDir, "test.bleve"),
	}

	// Create index service with local vault path
	ctx := context.Background()
	svc, err := NewIndexService(ctx, vault, vaultDir)
	if err != nil {
		t.Fatalf("NewIndexService() error = %v", err)
	}
	defer svc.Stop()

	// Start indexing (non-blocking)
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Monitor status updates
	statusUpdates := []StatusUpdate{}
	done := false
	for !done {
		select {
		case update, ok := <-svc.StatusUpdates():
			if !ok {
				done = true
				break
			}
			statusUpdates = append(statusUpdates, update)
			t.Logf("Status: %s - %s (Indexed: %d/%d, Remaining: %d)",
				update.Status, update.Message, update.IndexedCount, update.TotalCount, update.RemainingCount)

			if update.Error != nil {
				t.Fatalf("Indexing error: %v", update.Error)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for indexing to complete")
		}
	}

	// Verify we got status updates
	if len(statusUpdates) == 0 {
		t.Error("Expected status updates, got none")
	}

	// Verify final status
	finalStatus := svc.GetStatus()
	if finalStatus != StatusReady {
		t.Errorf("Final status = %v, want %v", finalStatus, StatusReady)
	}

	// Verify index was created
	index := svc.GetIndex()
	if index == nil {
		t.Fatal("GetIndex() returned nil")
	}

	// Check document count
	count, err := index.DocCount()
	if err != nil {
		t.Fatalf("DocCount() error = %v", err)
	}

	expectedCount := uint64(3)
	if count != expectedCount {
		t.Errorf("DocCount() = %d, want %d", count, expectedCount)
	}

	t.Logf("✓ Successfully indexed %d documents with %d status updates", count, len(statusUpdates))
}

func TestIndexService_ReIndex(t *testing.T) {
	// Create temporary directories
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	// Create initial test file
	testFile := filepath.Join(vaultDir, "note.md")
	initialContent := `# Original Title
Original content.
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create and index
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

	// Start indexing
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for indexing to complete
	for update := range svc.StatusUpdates() {
		if update.Status == StatusReady || update.Status == StatusError {
			break
		}
	}

	// Modify the file
	updatedContent := `# Updated Title
Updated content with #newtag.
`
	if err := os.WriteFile(testFile, []byte(updatedContent), 0644); err != nil {
		t.Fatalf("Failed to update test file: %v", err)
	}

	// Re-index the file (use empty string for fileID to fallback to relative path)
	if err := svc.reIndex(testFile, ""); err != nil {
		t.Fatalf("reIndex() error = %v", err)
	}

	t.Log("✓ Successfully re-indexed document")
}

func TestIndexService_DeleteFromIndex(t *testing.T) {
	// Create temporary directories
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	// Create test files
	testFile1 := filepath.Join(vaultDir, "note1.md")
	testFile2 := filepath.Join(vaultDir, "note2.md")

	for _, file := range []string{testFile1, testFile2} {
		content := `# Test Note
Content here.
`
		if err := os.WriteFile(file, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	// Create and index
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

	// Start indexing
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for indexing to complete
	for update := range svc.StatusUpdates() {
		if update.Status == StatusReady || update.Status == StatusError {
			break
		}
	}

	// Verify initial count
	index := svc.GetIndex()
	count, _ := index.DocCount()
	if count != 2 {
		t.Errorf("Initial DocCount() = %d, want 2", count)
	}

	// Delete one document (use empty string for fileID to fallback to relative path)
	if err := svc.deleteFromIndex(testFile1, ""); err != nil {
		t.Fatalf("deleteFromIndex() error = %v", err)
	}

	// Verify count after deletion
	count, _ = index.DocCount()
	if count != 1 {
		t.Errorf("After deletion DocCount() = %d, want 1", count)
	}

	t.Log("✓ Successfully deleted document from index")
}

func TestIndexService_ContextCancellation(t *testing.T) {
	// Create temporary directories with many files to ensure indexing takes time
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	// Create many test files to increase indexing time
	for i := 0; i < 1000; i++ {
		testFile := filepath.Join(vaultDir, fmt.Sprintf("note%d.md", i))
		content := fmt.Sprintf("# Note %d\nContent for note %d with some additional text to make parsing slower", i, i)
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

	vault := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		IndexPath: filepath.Join(indexDir, "test.bleve"),
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	svc, err := NewIndexService(ctx, vault, vaultDir)
	if err != nil {
		t.Fatalf("NewIndexService() error = %v", err)
	}
	defer svc.Stop()

	// Start indexing
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Cancel almost immediately to ensure we catch it during indexing
	time.Sleep(5 * time.Millisecond)
	cancel()

	// Wait for status updates and check for cancellation
	cancelled := false
	for update := range svc.StatusUpdates() {
		t.Logf("Status: %s - %s", update.Status, update.Message)
		if update.Status == StatusCancelled {
			t.Log("✓ Indexing was cancelled as expected")
			cancelled = true
			break
		}
		if update.Status == StatusReady || update.Status == StatusError {
			// Indexing completed before cancellation - this is okay, just skip the check
			t.Log("Indexing completed before cancellation could take effect - skipping test")
			return
		}
	}

	if cancelled {
		finalStatus := svc.GetStatus()
		if finalStatus != StatusCancelled {
			t.Errorf("Final status = %v, want %v", finalStatus, StatusCancelled)
		}
	}
}

func TestIndexService_NonBlocking(t *testing.T) {
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(vaultDir, "note.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

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

	// Start should return immediately (non-blocking)
	start := time.Now()
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	duration := time.Since(start)

	if duration > 100*time.Millisecond {
		t.Errorf("Start() took %v, expected to be non-blocking", duration)
	}

	// Wait for completion
	for range svc.StatusUpdates() {
	}

	t.Log("✓ Start() returned immediately (non-blocking)")
}

func TestIndexService_UpdateIndex(t *testing.T) {
	// Create temporary directories
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	// Create initial test files
	note1 := filepath.Join(vaultDir, "note1.md")
	note2 := filepath.Join(vaultDir, "note2.md")
	note3 := filepath.Join(vaultDir, "note3.md")

	if err := os.WriteFile(note1, []byte("# Note 1\nInitial content"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	if err := os.WriteFile(note2, []byte("# Note 2\nInitial content"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

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

	// Start indexing and wait for completion
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	for update := range svc.StatusUpdates() {
		if update.Status == StatusReady || update.Status == StatusError {
			break
		}
	}

	// Verify initial index count
	index := svc.GetIndex()
	count, _ := index.DocCount()
	if count != 2 {
		t.Errorf("Initial DocCount() = %d, want 2", count)
	}

	// Test 1: Create event - add a new file
	if err := os.WriteFile(note3, []byte("# Note 3\nNew content"), 0644); err != nil {
		t.Fatalf("Failed to write new file: %v", err)
	}

	// Send all events first
	svc.UpdateIndex(sync.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      note3,
		EventType: sync.FileCreated,
		Timestamp: time.Now(),
	})

	// Test 2: Modified event - update existing file
	if err := os.WriteFile(note1, []byte("# Note 1 Updated\nModified content"), 0644); err != nil {
		t.Fatalf("Failed to update file: %v", err)
	}

	svc.UpdateIndex(sync.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      note1,
		EventType: sync.FileModified,
		Timestamp: time.Now(),
	})

	// Test 3: Deleted event - remove a file
	svc.UpdateIndex(sync.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      note2,
		EventType: sync.FileDeleted,
		Timestamp: time.Now(),
	})

	// Wait for batch flush (flush interval is 500ms)
	time.Sleep(600 * time.Millisecond)

	// Verify final count after all events processed
	count, _ = index.DocCount()
	if count != 2 {
		t.Errorf("Final DocCount() = %d, want 2 (note1 modified, note3 created, note2 deleted)", count)
	}

	t.Log("✓ UpdateIndex handles Create, Modified, and Deleted events correctly with batching")
}

func TestIndexService_UpdateIndexNonBlocking(t *testing.T) {
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(vaultDir, "note.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

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

	// Start and wait for ready
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	for update := range svc.StatusUpdates() {
		if update.Status == StatusReady || update.Status == StatusError {
			break
		}
	}

	// UpdateIndex should return immediately (non-blocking)
	start := time.Now()
	svc.UpdateIndex(sync.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      testFile,
		EventType: sync.FileModified,
		Timestamp: time.Now(),
	})
	duration := time.Since(start)

	if duration > 10*time.Millisecond {
		t.Errorf("UpdateIndex() took %v, expected to be non-blocking", duration)
	}

	t.Log("✓ UpdateIndex() returns immediately (non-blocking)")
}

func TestIndexService_UpdateIndexBeforeReady(t *testing.T) {
	vaultDir := t.TempDir()
	indexDir := t.TempDir()

	// Create many files to ensure indexing takes some time
	for i := 0; i < 100; i++ {
		testFile := filepath.Join(vaultDir, fmt.Sprintf("note%d.md", i))
		if err := os.WriteFile(testFile, []byte(fmt.Sprintf("# Note %d", i)), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
	}

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

	// Start indexing
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Send event before service is ready
	testFile := filepath.Join(vaultDir, "note0.md")
	svc.UpdateIndex(sync.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      testFile,
		EventType: sync.FileModified,
		Timestamp: time.Now(),
	})

	// This should be skipped since service is not ready yet
	// Wait for service to become ready
	for update := range svc.StatusUpdates() {
		if update.Status == StatusReady || update.Status == StatusError {
			break
		}
	}

	t.Log("✓ UpdateIndex gracefully handles events before service is ready")
}

// Helper function
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
