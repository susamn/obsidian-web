package vault

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/db"
	"github.com/susamn/obsidian-web/internal/sse"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// Mock DB service for testing
type mockDBService struct {
	*db.DBService
	updateCalls int
	shouldFail  bool
	mu          sync.Mutex
	entries     map[string]*db.FileEntry
}

func newMockDBService() *mockDBService {
	return &mockDBService{
		entries: make(map[string]*db.FileEntry),
	}
}

func (m *mockDBService) GetFileEntryByPath(path string) (*db.FileEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.entries[path]; exists {
		return entry, nil
	}

	// Return a mock entry
	return &db.FileEntry{
		ID:   fmt.Sprintf("mock-id-%s", path),
		Name: filepath.Base(path),
		Path: path,
	}, nil
}

// Mock Index service
type mockIndexService struct {
	reindexCalls int
	deleteCalls  int
	mu           sync.Mutex
}

func (m *mockIndexService) ReIndexSync(path string, fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reindexCalls++
	return nil
}

func (m *mockIndexService) DeleteFromIndexSync(path string, fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls++
	return nil
}

// Mock Explorer service
type mockExplorerService struct {
	invalidateCalls int
	mu              sync.Mutex
}

func (m *mockExplorerService) InvalidateCacheSync(event syncpkg.FileChangeEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.invalidateCalls++
}

func TestWorker_ProcessEvent_Success(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create temp directory for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")

	// Create test file
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mockDB := newMockDBService()
	mockIndex := &mockIndexService{}
	mockExplorer := &mockExplorerService{}
	sseChannel := make(chan sse.Event, 10)
	var wg sync.WaitGroup

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		mockDB,
		mockIndex,
		mockExplorer,
		sseChannel,
	)

	// Create event
	event := syncpkg.FileChangeEvent{
		Path:      testFile,
		EventType: syncpkg.FileCreated,
		Timestamp: time.Now(),
	}

	// Process event
	worker.processEvent(event)

	// Wait a bit for processing
	time.Sleep(100 * time.Millisecond)

	// Verify index was called
	mockIndex.mu.Lock()
	indexCalls := mockIndex.reindexCalls
	mockIndex.mu.Unlock()
	if indexCalls != 1 {
		t.Errorf("Expected 1 index update call, got %d", indexCalls)
	}

	// Verify explorer was called
	mockExplorer.mu.Lock()
	explorerCalls := mockExplorer.invalidateCalls
	mockExplorer.mu.Unlock()
	if explorerCalls != 1 {
		t.Errorf("Expected 1 explorer invalidate call, got %d", explorerCalls)
	}

	// Verify SSE event was queued
	select {
	case evt := <-sseChannel:
		// Verify event has relative path and file ID
		relPath, _ := filepath.Rel(tmpDir, testFile)
		if evt.Path != relPath {
			t.Errorf("Expected relative path %s, got %s", relPath, evt.Path)
		}
		if evt.FileID == "" {
			t.Error("Expected FileID to be populated")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected SSE event to be queued")
	}
}

func TestWorker_ProcessEvent_Deleted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.md")

	mockDB := newMockDBService()
	mockIndex := &mockIndexService{}
	mockExplorer := &mockExplorerService{}
	sseChannel := make(chan sse.Event, 10)
	var wg sync.WaitGroup

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		mockDB,
		mockIndex,
		mockExplorer,
		sseChannel,
	)

	event := syncpkg.FileChangeEvent{
		Path:      testFile,
		EventType: syncpkg.FileDeleted,
		Timestamp: time.Now(),
	}

	worker.processEvent(event)
	time.Sleep(100 * time.Millisecond)

	// Verify delete from index was called
	mockIndex.mu.Lock()
	deleteCalls := mockIndex.deleteCalls
	mockIndex.mu.Unlock()
	if deleteCalls != 1 {
		t.Errorf("Expected 1 delete call, got %d", deleteCalls)
	}

	// Verify SSE event type
	select {
	case evt := <-sseChannel:
		if evt.Type != sse.EventFileDeleted {
			t.Errorf("Expected EventFileDeleted, got %s", evt.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected SSE event to be queued")
	}
}

func TestWorker_Metrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()

	mockDB := newMockDBService()
	mockIndex := &mockIndexService{}
	mockExplorer := &mockExplorerService{}
	sseChannel := make(chan sse.Event, 100)
	var wg sync.WaitGroup

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		mockDB,
		mockIndex,
		mockExplorer,
		sseChannel,
	)

	worker.Start()

	// Send 5 events
	for i := 0; i < 5; i++ {
		testFile := filepath.Join(tmpDir, fmt.Sprintf("test%d.md", i))

		// Create test file
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		worker.queue <- syncpkg.FileChangeEvent{
			Path:      testFile,
			EventType: syncpkg.FileCreated,
			Timestamp: time.Now(),
		}
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	metrics := worker.GetMetrics()

	if metrics.ProcessedCount != 5 {
		t.Errorf("Expected 5 processed events, got %d", metrics.ProcessedCount)
	}

	if metrics.FailedCount != 0 {
		t.Errorf("Expected 0 failed events, got %d", metrics.FailedCount)
	}

	cancel()
	wg.Wait()
}

func TestWorker_QueueDepth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()

	mockDB := newMockDBService()
	mockIndex := &mockIndexService{}
	mockExplorer := &mockExplorerService{}
	sseChannel := make(chan sse.Event, 100)
	var wg sync.WaitGroup

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		mockDB,
		mockIndex,
		mockExplorer,
		sseChannel,
	)

	// Add events to queue
	for i := 0; i < 10; i++ {
		worker.queue <- syncpkg.FileChangeEvent{
			Path:      filepath.Join(tmpDir, fmt.Sprintf("test%d.md", i)),
			EventType: syncpkg.FileCreated,
			Timestamp: time.Now(),
		}
	}

	depth := worker.GetQueueDepth()
	if depth != 10 {
		t.Errorf("Expected queue depth of 10, got %d", depth)
	}

	cancel()
}

func TestWorker_SecurityPathConversion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "subdir", "test.md")

	// Create subdirectory and file
	if err := os.MkdirAll(filepath.Dir(testFile), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mockDB := newMockDBService()
	mockIndex := &mockIndexService{}
	mockExplorer := &mockExplorerService{}
	sseChannel := make(chan sse.Event, 10)
	var wg sync.WaitGroup

	worker := NewWorker(
		0,
		"test-vault",
		tmpDir,
		ctx,
		&wg,
		mockDB,
		mockIndex,
		mockExplorer,
		sseChannel,
	)

	event := syncpkg.FileChangeEvent{
		Path:      testFile,
		EventType: syncpkg.FileCreated,
		Timestamp: time.Now(),
	}

	worker.processEvent(event)

	// Verify SSE event has relative path only
	select {
	case evt := <-sseChannel:
		// Path should be relative, NOT absolute
		if filepath.IsAbs(evt.Path) {
			t.Errorf("SECURITY: SSE event contains absolute path: %s", evt.Path)
		}

		// Should match expected relative path
		expectedPath := filepath.Join("subdir", "test.md")
		if evt.Path != expectedPath {
			t.Errorf("Expected relative path %s, got %s", expectedPath, evt.Path)
		}

		// Should contain vault ID
		if evt.VaultID != "test-vault" {
			t.Errorf("Expected vault ID 'test-vault', got %s", evt.VaultID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected SSE event")
	}
}
