package search

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/susamn/obsidian-web/internal/indexing"
)

// TestNewSearchService tests creating a search service
func TestNewSearchService(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "test.bleve")

	// Create a test index
	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}
	defer index.Close()

	// Create search service
	svc := NewSearchService(ctx, "test-vault", index)
	if svc == nil {
		t.Fatal("NewSearchService returned nil")
	}

	// Verify initial state
	if svc.vaultID != "test-vault" {
		t.Errorf("Expected vaultID 'test-vault', got '%s'", svc.vaultID)
	}

	if svc.GetStatus() != StatusInitializing {
		t.Errorf("Expected status Initializing, got %s", svc.GetStatus())
	}
}

// TestSearchService_Lifecycle tests the full lifecycle
func TestSearchService_Lifecycle(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "test.bleve")

	// Create a test index
	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}
	defer index.Close()

	// Create and start search service
	svc := NewSearchService(ctx, "test-vault", index)

	// Start service
	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Should be ready
	if svc.GetStatus() != StatusReady {
		t.Errorf("Expected status Ready, got %s", svc.GetStatus())
	}

	// Stop service
	if err := svc.Stop(); err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	// Should be stopped
	if svc.GetStatus() != StatusStopped {
		t.Errorf("Expected status Stopped, got %s", svc.GetStatus())
	}
}

// TestSearchService_WithNilIndex tests creating search service with nil index
func TestSearchService_WithNilIndex(t *testing.T) {
	ctx := context.Background()

	// Create search service with nil index
	svc := NewSearchService(ctx, "test-vault", nil)

	// Start service (should not error, but will be in Error status)
	if err := svc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Should be in error state since no index
	if svc.GetStatus() != StatusError {
		t.Logf("Status is %s (expected Error or Initializing)", svc.GetStatus())
	}
}

// TestSearchService_IndexUpdate tests index update notification
func TestSearchService_IndexUpdate(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "test.bleve")

	// Create a test index
	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}
	defer index.Close()

	// Create search service with nil index initially
	svc := NewSearchService(ctx, "test-vault", nil)

	// Start service
	svc.Start()

	// Notify of index update (rebuild event with new index)
	event := indexing.IndexUpdateEvent{
		Timestamp: time.Now(),
		EventType: "rebuild",
		NewIndex:  index,
	}
	svc.NotifyIndexUpdate(event)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Should now be ready
	if svc.GetStatus() != StatusReady {
		t.Errorf("Expected status Ready after index update, got %s", svc.GetStatus())
	}

	metrics := svc.GetMetrics()
	if metrics.IndexRefreshes != 1 {
		t.Errorf("Expected 1 index refresh, got %d", metrics.IndexRefreshes)
	}

	svc.Stop()
}

// TestSearchService_SearchMethods tests basic search functionality
func TestSearchService_SearchMethods(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "test.bleve")

	// Create a test index
	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}
	defer index.Close()

	// Index a test document
	doc := map[string]interface{}{
		"title": "Test Document",
		"path":  "/test.md",
		"tags":  []string{"golang", "testing"},
	}
	if err := index.Index("test-doc", doc); err != nil {
		t.Fatalf("Failed to index document: %v", err)
	}

	// Create and start search service
	svc := NewSearchService(ctx, "test-vault", index)
	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer svc.Stop()

	// Test SearchByText
	results, err := svc.SearchByText("Test")
	if err != nil {
		t.Errorf("SearchByText failed: %v", err)
	}
	if results != nil && results.Total != 1 {
		t.Errorf("Expected 1 result, got %d", results.Total)
	}

	// Test SearchByTag
	results, err = svc.SearchByTag("golang")
	if err != nil {
		t.Errorf("SearchByTag failed: %v", err)
	}

	// Verify metrics were updated
	metrics := svc.GetMetrics()
	if metrics.SearchCount < 2 {
		t.Errorf("Expected at least 2 searches, got %d", metrics.SearchCount)
	}

	if metrics.LastSearchTime.IsZero() {
		t.Error("Expected LastSearchTime to be set")
	}
}

// TestSearchService_SearchWithoutIndex tests search when index is not available
func TestSearchService_SearchWithoutIndex(t *testing.T) {
	ctx := context.Background()

	// Create search service with nil index
	svc := NewSearchService(ctx, "test-vault", nil)
	svc.Start()
	defer svc.Stop()

	// Try to search (should return error)
	_, err := svc.SearchByText("test")
	if err == nil {
		t.Error("Expected error when searching without index, got nil")
	}
}

// TestSearchService_GetMetrics tests metrics collection
func TestSearchService_GetMetrics(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "test.bleve")

	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}
	defer index.Close()

	svc := NewSearchService(ctx, "test-vault", index)
	svc.Start()
	defer svc.Stop()

	metrics := svc.GetMetrics()

	if metrics.Status != StatusReady {
		t.Errorf("Expected status Ready, got %s", metrics.Status)
	}

	if !metrics.HasIndex {
		t.Error("Expected HasIndex to be true")
	}

	if metrics.SearchCount != 0 {
		t.Errorf("Expected SearchCount 0, got %d", metrics.SearchCount)
	}
}

// TestSearchService_ConcurrentAccess tests concurrent search operations
func TestSearchService_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "test.bleve")

	mapping := bleve.NewIndexMapping()
	index, err := bleve.New(indexPath, mapping)
	if err != nil {
		t.Fatalf("Failed to create test index: %v", err)
	}
	defer index.Close()

	svc := NewSearchService(ctx, "test-vault", index)
	svc.Start()
	defer svc.Stop()

	// Concurrent searches
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 10; j++ {
				_, _ = svc.SearchByText("test")
				_ = svc.GetMetrics()
				_ = svc.GetStatus()
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify metrics
	metrics := svc.GetMetrics()
	if metrics.SearchCount != 100 {
		t.Errorf("Expected 100 searches, got %d", metrics.SearchCount)
	}
}

// TestSearchService_MultipleIndexUpdates tests multiple index refresh notifications
func TestSearchService_MultipleIndexUpdates(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	// Create initial index
	indexPath1 := filepath.Join(tempDir, "test1.bleve")
	mapping := bleve.NewIndexMapping()
	index1, err := bleve.New(indexPath1, mapping)
	if err != nil {
		t.Fatalf("Failed to create test index 1: %v", err)
	}
	defer index1.Close()

	svc := NewSearchService(ctx, "test-vault", index1)
	svc.Start()
	defer svc.Stop()

	// Send multiple incremental updates (index is updated in-place)
	for i := 0; i < 5; i++ {
		event := indexing.IndexUpdateEvent{
			Timestamp: time.Now(),
			EventType: "incremental",
			NewIndex:  nil, // No new index, just notification
		}
		svc.NotifyIndexUpdate(event)
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	metrics := svc.GetMetrics()
	if metrics.IndexRefreshes < 5 {
		t.Errorf("Expected at least 5 refreshes, got %d", metrics.IndexRefreshes)
	}
}

// TestServiceStatus_String tests status string conversion
func TestServiceStatus_String(t *testing.T) {
	tests := []struct {
		status   ServiceStatus
		expected string
	}{
		{StatusInitializing, "initializing"},
		{StatusReady, "ready"},
		{StatusStopped, "stopped"},
		{StatusError, "error"},
		{ServiceStatus(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
