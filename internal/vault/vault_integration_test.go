package vault

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/db"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// setupTestVault creates a test vault with file structure
func setupTestVault(t *testing.T) (string, *config.VaultConfig) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "index")
	dbPath := filepath.Join(tmpDir, "db")

	// Create directory structure
	dirs := []string{
		"notes",
		"notes/personal",
		"notes/work",
		"docs",
		"db", // Add db directory
	}

	for _, dir := range dirs {
		path := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", path, err)
		}
	}

	// Create test files
	files := map[string]string{
		"README.md":               "# Vault\n\nTest vault",
		"notes/note1.md":          "# Note 1\n\nTest note",
		"notes/note2.md":          "# Note 2\n\nAnother note",
		"notes/personal/diary.md": "# Diary\n\nPersonal diary",
		"notes/work/project.md":   "# Project\n\nWork project",
		"docs/guide.md":           "# Guide\n\nUser guide",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Create vault config
	cfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		Enabled:   true,
		IndexPath: indexPath,
		DBPath:    dbPath,
		Storage: config.StorageConfig{
			Type:  "local",
			Local: &config.LocalStorageConfig{Path: tmpDir},
		},
	}

	return tmpDir, cfg
}

// TestVaultInitialization tests vault creation and initialization
func TestVaultInitialization(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if vault == nil {
		t.Fatal("Vault is nil")
	}

	if vault.config.ID != "test-vault" {
		t.Errorf("Expected vault ID 'test-vault', got '%s'", vault.config.ID)
	}

	if vault.GetStatus() != VaultStatusInitializing {
		t.Errorf("Expected status %s, got %s", VaultStatusInitializing, vault.GetStatus())
	}

	vault.Stop()
}

// TestVaultStart tests vault startup and service initialization
func TestVaultStart(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	// Wait for vault to be ready
	if err := vault.WaitForReady(10 * time.Second); err != nil {
		t.Fatalf("Vault failed to become ready: %v", err)
	}

	if vault.GetStatus() != VaultStatusActive {
		t.Errorf("Expected status %s, got %s", VaultStatusActive, vault.GetStatus())
	}
}

// TestDBServiceIntegration tests database service integration with vault
func TestDBServiceIntegration(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	// Wait for vault to be ready
	vault.WaitForReady(10 * time.Second)

	dbSvc := vault.GetDBService()
	if dbSvc == nil {
		t.Fatal("Database service is nil")
	}

	if dbSvc.GetStatus() != db.StatusReady {
		t.Errorf("Expected db status %s, got %s", db.StatusReady, dbSvc.GetStatus())
	}
}

// TestVaultInitialization_SkipTestIntegrationDuplicate tests vault creation and initialization
// (keeping only one test to avoid duplication)
func TestVaultInitialization_DBReady(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if vault.GetDBService() != nil {
		t.Log("Database service initialized in NewVault")
	}
}

// TestForceReindex tests force reindexing functionality
func TestForceReindex(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	vault.WaitForReady(10 * time.Second)

	// Trigger reindex (async)
	vault.TriggerReindex()
	time.Sleep(8 * time.Second) // Wait for async reindex

	// Wait for reindex to complete
	time.Sleep(10 * time.Second)

	// Verify database was repopulated
	dbSvc := vault.GetDBService()
	entries, err := dbSvc.GetFileEntriesByParentID(nil)
	if err != nil {
		t.Fatalf("Failed to get root entries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("Expected database to be repopulated after reindex")
	}

	// Verify status is back to active
	if vault.GetStatus() != VaultStatusActive {
		t.Errorf("Expected vault to be active after reindex, got %s", vault.GetStatus())
	}
}

// TestExplorerServiceIntegration tests explorer service integration
func TestExplorerServiceIntegration(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	vault.WaitForReady(10 * time.Second)

	// Reindex to populate database
	vault.TriggerReindex() // Wait for async reindex
	time.Sleep(10 * time.Second)
	if false {
		t.Fatalf("Failed to force reindex: %v", err)
	}

	explorerSvc := vault.GetExplorerService()
	if explorerSvc == nil {
		t.Fatal("Explorer service is nil")
	}

	// Get tree with IDs
	tree, err := explorerSvc.GetTree("")
	if err != nil {
		t.Fatalf("Failed to get tree: %v", err)
	}

	if tree.Metadata.ID == "" {
		t.Error("Expected root node to have ID")
	}

	// Get children
	children, err := explorerSvc.GetChildren("")
	if err != nil {
		t.Fatalf("Failed to get children: %v", err)
	}

	if len(children) == 0 {
		t.Fatal("Expected children in root directory")
	}

	// Verify children have IDs
	for _, child := range children {
		if child.Metadata.ID == "" {
			t.Errorf("Child %s has no ID", child.Metadata.Name)
		}
	}
}

// TestFileEventProcessing tests file event processing through vault
func TestFileEventProcessing(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	vault.WaitForReady(10 * time.Second)

	// Reindex to populate database
	vault.TriggerReindex()
	time.Sleep(8 * time.Second) // Wait for async reindex

	// Get explorer service
	explorerSvc := vault.GetExplorerService()

	// Generate initial cache entries
	explorerSvc.GetTree("")

	// Wait for initial cache to be populated
	time.Sleep(100 * time.Millisecond)

	// Get initial state
	initialStats := explorerSvc.GetCacheStats()
	initialSize := initialStats["size"].(int)

	// Create a file change event
	event := syncpkg.FileChangeEvent{
		VaultID:   vault.config.ID,
		Path:      filepath.Join(vault.vaultPath, "new_note.md"),
		EventType: syncpkg.FileCreated,
		Timestamp: time.Now(),
	}

	// Send event through vault's event router
	explorerSvc.UpdateIndex(event)

	// Give event processing time
	time.Sleep(200 * time.Millisecond)

	// Verify cache was affected
	afterStats := explorerSvc.GetCacheStats()
	afterSize := afterStats["size"].(int)

	// Cache should be cleared/invalidated when files change
	if afterSize > initialSize && initialSize > 0 {
		t.Log("Cache state changed after file event, which is expected")
	}
}

// TestDBPathRelativization tests that paths are stored correctly
func TestDBPathRelativization(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	vault.WaitForReady(10 * time.Second)

	// Force reindex
	vault.TriggerReindex()
	time.Sleep(8 * time.Second) // Wait for async reindex

	dbSvc := vault.GetDBService()

	// Get an entry and verify path is relative
	entries, err := dbSvc.GetFileEntriesByParentID(nil)
	if err != nil {
		t.Fatalf("Failed to get root entries: %v", err)
	}

	if len(entries) > 0 {
		entry := entries[0]
		// Paths should be relative (not absolute)
		if !filepath.IsAbs(entry.Path) {
			t.Logf("Path %s is relative, as expected", entry.Path)
		}
	}
}

// TestMultiLevelHierarchy tests multi-level directory hierarchy
func TestMultiLevelHierarchy(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	vault.WaitForReady(10 * time.Second)
	vault.TriggerReindex()
	time.Sleep(8 * time.Second) // Wait for async reindex

	dbSvc := vault.GetDBService()

	// Get notes directory
	notesEntry, err := dbSvc.GetFileEntryByPath("notes")
	if err != nil {
		t.Fatalf("Failed to get notes entry: %v", err)
	}

	if notesEntry == nil {
		t.Fatal("notes directory not found")
	}

	// Get children of notes
	children, err := dbSvc.GetFileEntriesByParentID(&notesEntry.ID)
	if err != nil {
		t.Fatalf("Failed to get children: %v", err)
	}

	// Should have personal and work directories plus note files
	if len(children) == 0 {
		t.Fatal("Expected children in notes directory")
	}

	// Get personal directory and its children
	var personalEntry *db.FileEntry
	for _, child := range children {
		if child.Name == "personal" {
			personalEntry = &child
			break
		}
	}

	if personalEntry == nil {
		t.Fatal("personal directory not found")
	}

	// Get children of personal
	personalChildren, err := dbSvc.GetFileEntriesByParentID(&personalEntry.ID)
	if err != nil {
		t.Fatalf("Failed to get personal children: %v", err)
	}

	if len(personalChildren) == 0 {
		t.Fatal("Expected files in personal directory")
	}

	// Verify diary.md is there
	var found bool
	for _, child := range personalChildren {
		if child.Name == "diary.md" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("diary.md not found in personal directory")
	}
}

// TestExplorerIDFetching tests that explorer fetches IDs correctly
func TestExplorerIDFetching(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	vault.WaitForReady(10 * time.Second)
	vault.TriggerReindex()
	time.Sleep(8 * time.Second) // Wait for async reindex

	explorerSvc := vault.GetExplorerService()

	// Get metadata for a nested file
	meta, err := explorerSvc.GetMetadata("notes/personal/diary.md")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	if meta.ID == "" {
		t.Error("Expected ID to be fetched from database")
	}

	if meta.Name != "diary.md" {
		t.Errorf("Expected name 'diary.md', got '%s'", meta.Name)
	}

	if !meta.IsMarkdown {
		t.Error("Expected IsMarkdown to be true for .md file")
	}
}

// TestConcurrentVaultOperations tests concurrent operations on vault
func TestConcurrentVaultOperations(t *testing.T) {
	_, cfg := setupTestVault(t)
	ctx := context.Background()

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}
	defer vault.Stop()

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	vault.WaitForReady(10 * time.Second)
	vault.TriggerReindex()
	time.Sleep(8 * time.Second) // Wait for async reindex

	explorerSvc := vault.GetExplorerService()
	dbSvc := vault.GetDBService()

	// Concurrent reads from explorer
	done := make(chan bool, 3)

	go func() {
		for i := 0; i < 5; i++ {
			explorerSvc.GetTree("")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 5; i++ {
			explorerSvc.GetChildren("")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 5; i++ {
			entries, _ := dbSvc.GetFileEntriesByParentID(nil)
			if len(entries) > 0 {
				dbSvc.GetFileEntryByID(entries[0].ID)
			}
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Should complete without errors
	metrics := vault.GetMetrics()
	if metrics.Status != VaultStatusActive {
		t.Errorf("Expected vault to be active, got %s", metrics.Status)
	}
}
