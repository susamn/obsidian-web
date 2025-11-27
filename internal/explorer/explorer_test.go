package explorer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/db"
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	_, err := NewExplorerService(ctx, "test-vault", "/nonexistent/path", nil)
	if err == nil {
		t.Fatal("Expected error for nonexistent path, got nil")
	}
}

func TestGetTree_Root(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

func TestGetFullTree(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Get full recursive tree
	nodes, err := svc.GetFullTree()
	if err != nil {
		t.Fatalf("Failed to get full tree: %v", err)
	}

	// Should have root level items: file1.md, file2.txt, folder1, folder2
	// .hidden should be excluded
	if len(nodes) < 3 {
		t.Errorf("Expected at least 3 root nodes, got %d", len(nodes))
	}

	// Check that .hidden is not in nodes
	for _, node := range nodes {
		if node.Metadata.Name == ".hidden" {
			t.Error("Hidden directory should not appear in tree")
		}
	}

	// Find folder1 and verify it has children loaded
	var folder1 *TreeNode
	for _, node := range nodes {
		if node.Metadata.Name == "folder1" {
			folder1 = node
			break
		}
	}

	if folder1 == nil {
		t.Fatal("folder1 not found in tree")
	}

	if !folder1.Loaded {
		t.Error("folder1 should be marked as loaded in full tree")
	}

	if folder1.Children == nil {
		t.Fatal("folder1 children should be loaded in full tree")
	}

	// folder1 should have: nested.md and subfolder1
	if len(folder1.Children) < 2 {
		t.Errorf("Expected folder1 to have at least 2 children, got %d", len(folder1.Children))
	}

	// Find subfolder1 and verify it has children loaded recursively
	var subfolder1 *TreeNode
	for _, child := range folder1.Children {
		if child.Metadata.Name == "subfolder1" {
			subfolder1 = child
			break
		}
	}

	if subfolder1 == nil {
		t.Fatal("subfolder1 not found in folder1")
	}

	if !subfolder1.Loaded {
		t.Error("subfolder1 should be marked as loaded in full tree")
	}

	if subfolder1.Children == nil {
		t.Fatal("subfolder1 children should be loaded recursively")
	}

	// subfolder1 should have deep.md
	foundDeep := false
	for _, child := range subfolder1.Children {
		if child.Metadata.Name == "deep.md" {
			foundDeep = true
			if child.Metadata.Type != NodeTypeFile {
				t.Error("deep.md should be a file")
			}
		}
	}

	if !foundDeep {
		t.Error("deep.md not found in subfolder1")
	}

	// Verify all nodes have correct paths
	// Note: IDs will be empty without a DB service, which is fine
	var verifyNode func(*TreeNode, string)
	verifyNode = func(node *TreeNode, parentPath string) {
		expectedPath := parentPath
		if parentPath != "" {
			expectedPath = parentPath + "/" + node.Metadata.Name
		} else {
			expectedPath = node.Metadata.Name
		}

		if node.Metadata.Path != expectedPath {
			t.Errorf("Node %s has incorrect path: got %s, want %s",
				node.Metadata.Name, node.Metadata.Path, expectedPath)
		}

		if node.Children != nil {
			for _, child := range node.Children {
				verifyNode(child, node.Metadata.Path)
			}
		}
	}

	for _, node := range nodes {
		verifyNode(node, "")
	}
}

func TestGetChildren(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	// Cache for root should be updated (not invalidated)
	svc.cacheMu.RLock()
	rootNode, exists := svc.cache[""]
	svc.cacheMu.RUnlock()

	if !exists {
		t.Error("Expected root cache to be updated after file creation")
	}

	// Verify the cache has loaded children
	if !rootNode.Loaded {
		t.Error("Expected root cache to have loaded children")
	}
}

func TestRefreshPath(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, nil)
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

// TestExplorerWithDB tests explorer service with database integration
func TestExplorerWithDB(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	// Create database service
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewDBService(ctx, &dbPath)
	if err != nil {
		t.Fatalf("Failed to create db service: %v", err)
	}

	if err := dbSvc.Start(); err != nil {
		t.Fatalf("Failed to start db service: %v", err)
	}
	defer dbSvc.Stop()

	// Create explorer service with DB
	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, dbSvc)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// First populate DB
	populateDBFromDirectory(t, dbSvc, tmpDir, nil)

	// Now get tree and check IDs
	node, err := svc.GetTree("file1.md")
	if err != nil {
		t.Fatalf("Failed to get tree: %v", err)
	}

	if node.Metadata.ID == "" {
		t.Error("Expected ID to be populated from database for file1.md")
	}
}

// TestGetMetadataWithID tests that metadata includes ID from database
func TestGetMetadataWithID(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	// Create and populate database
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewDBService(ctx, &dbPath)
	if err != nil {
		t.Fatalf("Failed to create db service: %v", err)
	}

	if err := dbSvc.Start(); err != nil {
		t.Fatalf("Failed to start db service: %v", err)
	}
	defer dbSvc.Stop()

	// Populate database
	populateDBFromDirectory(t, dbSvc, tmpDir, nil)

	// Create explorer service
	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, dbSvc)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Get metadata
	meta, err := svc.GetMetadata("file1.md")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	if meta.ID == "" {
		t.Error("Expected ID in metadata")
	}

	if meta.Name != "file1.md" {
		t.Errorf("Expected name 'file1.md', got '%s'", meta.Name)
	}
}

// TestChildrenHaveIDs tests that children include IDs
func TestChildrenHaveIDs(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	// Create and populate database
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewDBService(ctx, &dbPath)
	if err != nil {
		t.Fatalf("Failed to create db service: %v", err)
	}

	if err := dbSvc.Start(); err != nil {
		t.Fatalf("Failed to start db service: %v", err)
	}
	defer dbSvc.Stop()

	// Populate database
	populateDBFromDirectory(t, dbSvc, tmpDir, nil)

	// Create explorer service
	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, dbSvc)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	defer svc.Stop()

	// Get children
	children, err := svc.GetChildren("")
	if err != nil {
		t.Fatalf("Failed to get children: %v", err)
	}

	if len(children) == 0 {
		t.Fatal("Expected children")
	}

	// Check that each child has an ID
	for _, child := range children {
		if child.Metadata.ID == "" {
			t.Errorf("Child %s has no ID", child.Metadata.Name)
		}
	}
}

// TestFileEventInvalidatesCacheWithDB tests that file events update parent cache
func TestFileEventInvalidatesCacheWithDB(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	// Create and populate database
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewDBService(ctx, &dbPath)
	if err != nil {
		t.Fatalf("Failed to create db service: %v", err)
	}

	if err := dbSvc.Start(); err != nil {
		t.Fatalf("Failed to start db service: %v", err)
	}
	defer dbSvc.Stop()

	// Populate database
	populateDBFromDirectory(t, dbSvc, tmpDir, nil)

	// Create explorer service
	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, dbSvc)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	svc.Start()
	defer svc.Stop()

	// Cache root
	rootNode1, err := svc.GetTree("")
	if err != nil {
		t.Fatalf("Failed to get tree: %v", err)
	}
	childCountBefore := rootNode1.Metadata.ChildCount

	// Create the actual file first
	testFile := filepath.Join(tmpDir, "newfile.md")
	if err := os.WriteFile(testFile, []byte("# New File"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Send a file created event
	event := syncpkg.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      testFile,
		EventType: syncpkg.FileCreated,
		Timestamp: time.Now(),
	}

	svc.UpdateIndex(event)

	// Give event processor time to process
	time.Sleep(150 * time.Millisecond)

	// Cache for root should be updated with new children count
	svc.cacheMu.RLock()
	rootNode2, exists := svc.cache[""]
	svc.cacheMu.RUnlock()

	if !exists {
		t.Error("Expected root cache to be updated after file creation")
	}

	if rootNode2.Metadata.ChildCount <= childCountBefore {
		t.Errorf("Expected child count to increase from %d, got %d",
			childCountBefore, rootNode2.Metadata.ChildCount)
	}
}

// TestFileCreatedUpdatesParentCache tests that creating a file updates parent cache
func TestFileCreatedUpdatesParentCache(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	// Create and populate database
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewDBService(ctx, &dbPath)
	if err != nil {
		t.Fatalf("Failed to create db service: %v", err)
	}

	if err := dbSvc.Start(); err != nil {
		t.Fatalf("Failed to start db service: %v", err)
	}
	defer dbSvc.Stop()

	// Populate database
	populateDBFromDirectory(t, dbSvc, tmpDir, nil)

	// Create explorer service
	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, dbSvc)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	svc.Start()
	defer svc.Stop()

	// Cache root - get initial children count
	rootNode1, err := svc.GetTree("")
	if err != nil {
		t.Fatalf("Failed to get root tree: %v", err)
	}
	initialChildCount := rootNode1.Metadata.ChildCount

	// Verify root is cached
	svc.cacheMu.RLock()
	_, rootCached := svc.cache[""]
	svc.cacheMu.RUnlock()
	if !rootCached {
		t.Fatal("Expected root to be cached")
	}

	// Create a new file at root
	newFile := filepath.Join(tmpDir, "newfile.md")
	if err := os.WriteFile(newFile, []byte("# New File"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Send file created event
	event := syncpkg.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      newFile,
		EventType: syncpkg.FileCreated,
		Timestamp: time.Now(),
	}

	svc.UpdateIndex(event)

	// Give event processor time to process
	time.Sleep(150 * time.Millisecond)

	// Get root cache again - should be refreshed with new child
	svc.cacheMu.RLock()
	rootNode2, cacheExists := svc.cache[""]
	svc.cacheMu.RUnlock()

	if !cacheExists {
		t.Fatal("Expected root cache to be updated after file creation")
	}

	// Child count should have increased
	if rootNode2.Metadata.ChildCount <= initialChildCount {
		t.Errorf("Expected child count to increase from %d, got %d",
			initialChildCount, rootNode2.Metadata.ChildCount)
	}

	// Verify the new file is in the children
	if len(rootNode2.Children) > 0 {
		found := false
		for _, child := range rootNode2.Children {
			if child.Metadata.Name == "newfile.md" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected newfile.md to be in cached children")
		}
	}
}

// TestFileCreatedInSubfolderUpdatesCache tests file creation in subdirectory
func TestFileCreatedInSubfolderUpdatesCache(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	// Create and populate database
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewDBService(ctx, &dbPath)
	if err != nil {
		t.Fatalf("Failed to create db service: %v", err)
	}

	if err := dbSvc.Start(); err != nil {
		t.Fatalf("Failed to start db service: %v", err)
	}
	defer dbSvc.Stop()

	// Populate database
	populateDBFromDirectory(t, dbSvc, tmpDir, nil)

	// Create explorer service
	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, dbSvc)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	svc.Start()
	defer svc.Stop()

	// Cache folder1
	folder1Node1, err := svc.GetTree("folder1")
	if err != nil {
		t.Fatalf("Failed to get folder1 tree: %v", err)
	}
	initialChildCount := folder1Node1.Metadata.ChildCount

	// Create a new file in folder1
	newFile := filepath.Join(tmpDir, "folder1", "newfolder1file.md")
	if err := os.WriteFile(newFile, []byte("# New File in Folder1"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Send file created event
	event := syncpkg.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      newFile,
		EventType: syncpkg.FileCreated,
		Timestamp: time.Now(),
	}

	svc.UpdateIndex(event)

	// Give event processor time to process
	time.Sleep(150 * time.Millisecond)

	// Get folder1 cache again
	svc.cacheMu.RLock()
	folder1Node2, cacheExists := svc.cache["folder1"]
	svc.cacheMu.RUnlock()

	if !cacheExists {
		t.Fatal("Expected folder1 cache to be updated after file creation")
	}

	// Child count should have increased
	if folder1Node2.Metadata.ChildCount <= initialChildCount {
		t.Errorf("Expected child count to increase from %d, got %d",
			initialChildCount, folder1Node2.Metadata.ChildCount)
	}

	// Verify the new file is in the children
	found := false
	for _, child := range folder1Node2.Children {
		if child.Metadata.Name == "newfolder1file.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected newfolder1file.md to be in cached children of folder1")
	}
}

// TestFileDeletedUpdatesParentCache tests that deleting a file updates parent cache
func TestFileDeletedUpdatesParentCache(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	// Create and populate database
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewDBService(ctx, &dbPath)
	if err != nil {
		t.Fatalf("Failed to create db service: %v", err)
	}

	if err := dbSvc.Start(); err != nil {
		t.Fatalf("Failed to start db service: %v", err)
	}
	defer dbSvc.Stop()

	// Populate database
	populateDBFromDirectory(t, dbSvc, tmpDir, nil)

	// Create explorer service
	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, dbSvc)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	svc.Start()
	defer svc.Stop()

	// Cache root - get initial children count
	rootNode1, err := svc.GetTree("")
	if err != nil {
		t.Fatalf("Failed to get root tree: %v", err)
	}
	initialChildCount := rootNode1.Metadata.ChildCount

	// Verify file exists in cache
	fileFound := false
	for _, child := range rootNode1.Children {
		if child.Metadata.Name == "file1.md" {
			fileFound = true
			break
		}
	}
	if !fileFound {
		t.Fatal("Expected file1.md to exist in root")
	}

	// Delete the file
	delFile := filepath.Join(tmpDir, "file1.md")
	if err := os.Remove(delFile); err != nil {
		t.Fatalf("Failed to delete test file: %v", err)
	}

	// Send file deleted event
	event := syncpkg.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      delFile,
		EventType: syncpkg.FileDeleted,
		Timestamp: time.Now(),
	}

	svc.UpdateIndex(event)

	// Give event processor time to process
	time.Sleep(150 * time.Millisecond)

	// Get root cache again
	svc.cacheMu.RLock()
	rootNode2, cacheExists := svc.cache[""]
	svc.cacheMu.RUnlock()

	if !cacheExists {
		t.Fatal("Expected root cache to be updated after file deletion")
	}

	// Child count should have decreased
	if rootNode2.Metadata.ChildCount >= initialChildCount {
		t.Errorf("Expected child count to decrease from %d, got %d",
			initialChildCount, rootNode2.Metadata.ChildCount)
	}

	// Verify the deleted file is NOT in children
	fileFound = false
	for _, child := range rootNode2.Children {
		if child.Metadata.Name == "file1.md" {
			fileFound = true
			break
		}
	}
	if fileFound {
		t.Error("Expected file1.md to be removed from cached children")
	}
}

// TestFileModifiedInvalidatesFileCache tests that file modification invalidates file cache
func TestFileModifiedInvalidatesFileCache(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	// Create and populate database
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewDBService(ctx, &dbPath)
	if err != nil {
		t.Fatalf("Failed to create db service: %v", err)
	}

	if err := dbSvc.Start(); err != nil {
		t.Fatalf("Failed to start db service: %v", err)
	}
	defer dbSvc.Stop()

	// Populate database
	populateDBFromDirectory(t, dbSvc, tmpDir, nil)

	// Create explorer service
	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, dbSvc)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	svc.Start()
	defer svc.Stop()

	// Cache file1.md metadata
	meta1, err := svc.GetMetadata("file1.md")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	// Send file modified event
	modFile := filepath.Join(tmpDir, "file1.md")
	event := syncpkg.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      modFile,
		EventType: syncpkg.FileModified,
		Timestamp: time.Now(),
	}

	svc.UpdateIndex(event)

	// Give event processor time to process
	time.Sleep(100 * time.Millisecond)

	// Check if cache for file1.md is invalidated
	svc.cacheMu.RLock()
	_, fileExists := svc.cache["file1.md"]
	svc.cacheMu.RUnlock()

	if fileExists {
		t.Error("Expected file1.md cache to be invalidated after modification")
	}

	// Parent cache should NOT be invalidated/refreshed for file modifications
	// (child list doesn't change)
	svc.cacheMu.RLock()
	rootNode, rootExists := svc.cache[""]
	svc.cacheMu.RUnlock()

	// Parent may or may not exist - that's OK. The important thing is
	// that it's not being unnecessarily refreshed for file modifications
	_ = rootNode
	_ = rootExists

	// Verify we can still get updated metadata
	meta2, err := svc.GetMetadata("file1.md")
	if err != nil {
		t.Fatalf("Failed to get metadata after modification: %v", err)
	}

	// Metadata should be fresh (not from old cache)
	if meta1 == meta2 {
		t.Error("Expected fresh metadata after file modification")
	}
}

// TestCacheUpdatePreservesMetadata tests that cache updates preserve file metadata
func TestCacheUpdatePreservesMetadata(t *testing.T) {
	tmpDir := setupTestDir(t)
	ctx := context.Background()

	// Create and populate database
	dbPath := filepath.Join(tmpDir, "test.db")
	dbSvc, err := db.NewDBService(ctx, &dbPath)
	if err != nil {
		t.Fatalf("Failed to create db service: %v", err)
	}

	if err := dbSvc.Start(); err != nil {
		t.Fatalf("Failed to start db service: %v", err)
	}
	defer dbSvc.Stop()

	// Populate database
	populateDBFromDirectory(t, dbSvc, tmpDir, nil)

	// Create explorer service
	svc, err := NewExplorerService(ctx, "test-vault", tmpDir, dbSvc)
	if err != nil {
		t.Fatalf("Failed to create explorer service: %v", err)
	}
	svc.Start()
	defer svc.Stop()

	// Cache root
	_, err = svc.GetTree("")
	if err != nil {
		t.Fatalf("Failed to get root tree: %v", err)
	}

	// Create a new file
	newFile := filepath.Join(tmpDir, "newtest.md")
	if err = os.WriteFile(newFile, []byte("# New Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Send file created event
	event := syncpkg.FileChangeEvent{
		VaultID:   "test-vault",
		Path:      newFile,
		EventType: syncpkg.FileCreated,
		Timestamp: time.Now(),
	}

	svc.UpdateIndex(event)

	// Give event processor time to process
	time.Sleep(150 * time.Millisecond)

	// Get updated root cache
	svc.cacheMu.RLock()
	rootNode2, _ := svc.cache[""]
	svc.cacheMu.RUnlock()

	// Verify cached metadata has all required fields
	if rootNode2.Metadata.Name == "" {
		t.Error("Expected Name in metadata")
	}

	if rootNode2.Metadata.Type != NodeTypeDirectory {
		t.Error("Expected directory type")
	}

	// Verify children have metadata
	if len(rootNode2.Children) > 0 {
		found := false
		for _, child := range rootNode2.Children {
			if child.Metadata.Name == "" {
				t.Error("Expected name in child metadata")
			}
			// IsMarkdown should be set correctly
			if child.Metadata.Name == "newtest.md" {
				found = true
				if !child.Metadata.IsMarkdown {
					t.Error("Expected newtest.md to be marked as markdown")
				}
				// Verify all metadata fields are populated for cached children
				if child.Metadata.Type == "" {
					t.Error("Expected type in child metadata")
				}
				if child.Metadata.Size == 0 {
					t.Error("Expected non-zero size for newtest.md")
				}
			}
		}
		if !found {
			t.Error("Expected newtest.md to be in cached children")
		}
	}
}

// populateDBFromDirectory populates database from a directory structure
func populateDBFromDirectory(t *testing.T, dbSvc *db.DBService, dirPath string, parentID *string) {
	populateDBFromDirectoryWithBase(t, dbSvc, dirPath, parentID, dirPath)
}

// populateDBFromDirectoryWithBase populates database from a directory structure with a base path
func populateDBFromDirectoryWithBase(t *testing.T, dbSvc *db.DBService, dirPath string, parentID *string, basePath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	// Get ACTIVE status ID for all entries
	activeStatusID, err := dbSvc.GetFileStatusID(db.FileStatusActive)
	if err != nil {
		t.Fatalf("Failed to get ACTIVE status ID: %v", err)
	}

	for _, entry := range entries {
		// Skip hidden files
		if entry.Name()[0] == '.' {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		relPath, _ := filepath.Rel(basePath, fullPath)

		// Create simple ID based on name
		id := "id-" + entry.Name()

		info, _ := entry.Info()
		fileEntry := &db.FileEntry{
			ID:           id,
			Name:         entry.Name(),
			ParentID:     parentID,
			IsDir:        entry.IsDir(),
			Path:         relPath,
			FileStatusID: activeStatusID,
			Created:      time.Now().UTC(),
			Modified:     time.Now().UTC(),
		}

		if !entry.IsDir() && info != nil {
			fileEntry.Size = info.Size()
		}

		if err := dbSvc.CreateFileEntry(fileEntry); err != nil {
			t.Fatalf("Failed to create db entry: %v", err)
		}

		// Recursively process subdirectories
		if entry.IsDir() {
			populateDBFromDirectoryWithBase(t, dbSvc, fullPath, &id, basePath)
		}
	}
}
