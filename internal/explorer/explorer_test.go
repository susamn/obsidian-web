package explorer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// setupTestDir creates a temporary test directory structure
func setupTestDir(t *testing.T) string {
	tmpDir := t.TempDir()

	// Create directory structure
	dirs := []string{
		"folder1",
		"folder1/subfolder1",
		"folder2",
		".hidden", // Should be ignored
	}

	for _, dir := range dirs {
		path := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", path, err)
		}
	}

	// Create test files
	files := map[string]string{
		"file1.md":                   "# File 1\n\nThis is a test file.",
		"file2.txt":                  "Not a markdown file",
		"folder1/nested.md":          "# Nested\n\nNested markdown file.",
		"folder1/subfolder1/deep.md": "# Deep\n\nDeep nested file.",
		"folder2/another.md":         "# Another\n\nAnother file.",
		".hidden/hidden.md":          "Should not appear",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	return tmpDir
}

func TestNewExplorerService(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}

	if svc == nil {
		t.Fatal("Explorer service is nil")
	}

	if svc.vaultID != "test-vault" {
		t.Errorf("Expected vault ID 'test-vault', got '%s'", svc.vaultID)
	}

	if svc.vaultPath != tmpDir {
		t.Errorf("Expected vault path '%s', got '%s'", tmpDir, svc.vaultPath)
	}

	svc.Stop()
}

func TestNewExplorerService_InvalidPath(t *testing.T) {
	ctx := context.Background()

	_, err := NewExplorerService(ctx, "test-vault", "/nonexistent/path")
	if err == nil {
		t.Fatal("Expected error for nonexistent path, got nil")
	}
}

func TestGetTree_Root(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Get root tree
	node, err := svc.GetTree("")
	if err != nil {
		t.Fatalf("Failed to get root tree: %v", err)
	}

	if node.Metadata.Type != NodeTypeDirectory {
		t.Errorf("Expected directory, got %s", node.Metadata.Type)
	}

	if node.Metadata.Path != "" {
		t.Errorf("Expected empty path for root, got '%s'", node.Metadata.Path)
	}

	if !node.Metadata.HasChildren {
		t.Error("Expected root to have children")
	}
}

func TestGetChildren(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Get children of root
	children, err := svc.GetChildren("")
	if err != nil {
		t.Fatalf("Failed to get children: %v", err)
	}

	// Should have: file1.md, file2.txt, folder1, folder2
	// .hidden should be excluded
	if len(children) < 3 {
		t.Errorf("Expected at least 3 children, got %d", len(children))
	}

	// Check that .hidden is not in children
	for _, child := range children {
		if child.Metadata.Name == ".hidden" {
			t.Error("Hidden directory should not appear in children")
		}
	}

	// Check for expected items
	foundFolder1 := false
	foundFile1 := false

	for _, child := range children {
		if child.Metadata.Name == "folder1" {
			foundFolder1 = true
			if child.Metadata.Type != NodeTypeDirectory {
				t.Error("folder1 should be a directory")
			}
		}
		if child.Metadata.Name == "file1.md" {
			foundFile1 = true
			if child.Metadata.Type != NodeTypeFile {
				t.Error("file1.md should be a file")
			}
			if !child.Metadata.IsMarkdown {
				t.Error("file1.md should be marked as markdown")
			}
		}
	}

	if !foundFolder1 {
		t.Error("folder1 not found in children")
	}
	if !foundFile1 {
		t.Error("file1.md not found in children")
	}
}

func TestGetChildren_Nested(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Get children of folder1
	children, err := svc.GetChildren("folder1")
	if err != nil {
		t.Fatalf("Failed to get children of folder1: %v", err)
	}

	// Should have: nested.md, subfolder1
	if len(children) != 2 {
		t.Errorf("Expected 2 children in folder1, got %d", len(children))
	}
}

func TestGetMetadata(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Get metadata for a file
	meta, err := svc.GetMetadata("file1.md")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	if meta.Name != "file1.md" {
		t.Errorf("Expected name 'file1.md', got '%s'", meta.Name)
	}

	if meta.Type != NodeTypeFile {
		t.Errorf("Expected file type, got %s", meta.Type)
	}

	if !meta.IsMarkdown {
		t.Error("Expected IsMarkdown to be true")
	}

	if meta.Size == 0 {
		t.Error("Expected non-zero file size")
	}
}

func TestValidatePath_Traversal(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Test directory traversal prevention
	tests := []string{
		"../etc/passwd",
		"folder1/../../etc",
		"./../../secrets",
	}

	for _, path := range tests {
		_, err := svc.validatePath(path)
		if err == nil {
			t.Errorf("Expected error for path '%s', got nil", path)
		}
	}
}

func TestValidatePath_Valid(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Test valid paths
	tests := map[string]string{
		"folder1":            "folder1",
		"folder1/nested.md":  "folder1/nested.md",
		"":                   "",
		".":                  "",
		"./folder1":          "folder1",
		"folder1/subfolder1": "folder1/subfolder1",
	}

	for input, expected := range tests {
		clean, err := svc.validatePath(input)
		if err != nil {
			t.Errorf("Unexpected error for path '%s': %v", input, err)
		}
		if clean != expected {
			t.Errorf("For path '%s', expected '%s', got '%s'", input, expected, clean)
		}
	}
}

func TestCaching(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// First call - should cache
	node1, err := svc.GetTree("folder1")
	if err != nil {
		t.Fatalf("Failed to get tree: %v", err)
	}

	// Check cache
	svc.cacheMu.RLock()
	_, exists := svc.cache["folder1"]
	svc.cacheMu.RUnlock()

	if !exists {
		t.Error("Expected node to be cached")
	}

	// Second call - should use cache
	node2, err := svc.GetTree("folder1")
	if err != nil {
		t.Fatalf("Failed to get tree from cache: %v", err)
	}

	if node1.Metadata.CachedAt != node2.Metadata.CachedAt {
		t.Error("Expected cached node to be returned with same timestamp")
	}
}

func TestInvalidateCache(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Cache a node
	_, err = svc.GetTree("folder1")
	if err != nil {
		t.Fatalf("Failed to get tree: %v", err)
	}

	// Invalidate cache
	svc.invalidateCache("folder1")

	// Check cache
	svc.cacheMu.RLock()
	_, exists := svc.cache["folder1"]
	svc.cacheMu.RUnlock()

	if exists {
		t.Error("Expected cache to be invalidated")
	}
}

func TestEventHandling(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	svc.Start()
	defer svc.Stop()

	// Cache root
	_, err = svc.GetTree("")
	if err != nil {
		t.Fatalf("Failed to get tree: %v", err)
	}

	// Send a file created event
	testFile := filepath.Join(tmpDir, "newfile.md")
	event := syncpkg.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      testFile,
		EventType: syncpkg.FileCreated,
		Timestamp: time.Now(),
	}

	svc.UpdateIndex(event)

	// Give event processor time to process
	time.Sleep(100 * time.Millisecond)

	// Cache for root should be invalidated
	svc.cacheMu.RLock()
	_, exists := svc.cache[""]
	svc.cacheMu.RUnlock()

	if exists {
		t.Error("Expected root cache to be invalidated after file creation")
	}
}

func TestRefreshPath(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Refresh a path
	err = svc.RefreshPath("folder1")
	if err != nil {
		t.Fatalf("Failed to refresh path: %v", err)
	}

	// Check that it's now cached
	svc.cacheMu.RLock()
	_, exists := svc.cache["folder1"]
	svc.cacheMu.RUnlock()

	if !exists {
		t.Error("Expected path to be cached after refresh")
	}
}

func TestClearCache(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Cache multiple nodes
	svc.GetTree("")
	svc.GetTree("folder1")
	svc.GetTree("folder2")

	// Clear cache
	svc.ClearCache()

	// Check cache is empty
	svc.cacheMu.RLock()
	size := len(svc.cache)
	svc.cacheMu.RUnlock()

	if size != 0 {
		t.Errorf("Expected empty cache, got %d entries", size)
	}
}

func TestGetCacheStats(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Get stats
	stats := svc.GetCacheStats()

	if stats["vault_id"] != "test-vault" {
		t.Errorf("Expected vault_id 'test-vault', got '%v'", stats["vault_id"])
	}

	if stats["size"].(int) != 0 {
		t.Errorf("Expected initial size 0, got %v", stats["size"])
	}

	// Cache something
	svc.GetTree("")

	stats = svc.GetCacheStats()
	if stats["size"].(int) != 1 {
		t.Errorf("Expected size 1 after caching, got %v", stats["size"])
	}
}
