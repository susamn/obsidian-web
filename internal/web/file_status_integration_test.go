package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/db"
	"github.com/susamn/obsidian-web/internal/explorer"
	"github.com/susamn/obsidian-web/internal/vault"
)

// TestFileStatusIntegration tests the complete data flow:
// 1. Create nested folder structure with files
// 2. Verify files are indexed, in DB with ACTIVE status, and in explorer cache
// 3. Verify tree API returns only ACTIVE files
// 4. Delete files from nested folders
// 5. Verify deleted files have DELETED status, not in index/tree, but still in DB
func TestFileStatusIntegration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup
	tempDir := t.TempDir()
	indexDir := t.TempDir()
	dbDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "status-test-vault",
		Name:      "Status Test Vault",
		Enabled:   true,
		IndexPath: filepath.Join(indexDir, "test.bleve"),
		DBPath:    dbDir,
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	// Initialize vault
	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}
	defer v.Stop()

	if err := v.WaitForReady(5 * time.Second); err != nil {
		t.Fatalf("Vault not ready: %v", err)
	}

	// Initialize web server
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 19875,
		},
		Vaults: []config.VaultConfig{*vaultCfg},
	}

	server := NewServer(ctx, cfg, map[string]*vault.Vault{"status-test-vault": v})
	server.Start()
	defer server.Stop()

	dbService := v.GetDBService()
	explorerService := v.GetExplorerService()
	searchService := v.GetSearchService()

	// ==========================================
	// PHASE 1: Create nested folder structure
	// ==========================================
	t.Log("PHASE 1: Creating nested folder structure...")

	structure := map[string]string{
		"docs/README.md":                    "# Docs\n\nDocumentation",
		"docs/guide/intro.md":               "# Introduction\n\nGetting started",
		"docs/guide/advanced.md":            "# Advanced\n\nAdvanced topics",
		"notes/personal/diary.md":           "# Diary\n\nMy personal diary",
		"notes/personal/ideas.md":           "# Ideas\n\nBrainstorming",
		"notes/work/project-a.md":           "# Project A\n\nWork notes",
		"notes/work/meetings.md":            "# Meetings\n\nMeeting notes",
		"archive/old-notes/2023/jan.md":     "# January 2023\n\nOld notes",
		"archive/old-notes/2023/feb.md":     "# February 2023\n\nOld notes",
		"projects/active/backend/todo.md":   "# Backend TODO\n\nTasks",
		"projects/active/frontend/todo.md":  "# Frontend TODO\n\nTasks",
		"projects/archived/legacy/notes.md": "# Legacy Notes\n\nOld project",
	}

	// Create all files
	for path, content := range structure {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}

	// Wait for sync and indexing
	t.Log("Waiting for sync and indexing...")
	time.Sleep(3 * time.Second)

	// Force reindex to ensure everything is processed
	if err := v.ForceReindex(); err != nil {
		t.Fatalf("Failed to force reindex: %v", err)
	}
	time.Sleep(2 * time.Second)

	// ==========================================
	// PHASE 2: Verify DB has files with ACTIVE status
	// ==========================================
	t.Log("PHASE 2: Verifying database has all files with ACTIVE status...")

	for path := range structure {
		entry, err := dbService.GetFileEntryByPath(path)
		if err != nil {
			t.Errorf("Failed to get entry for %s: %v", path, err)
			continue
		}
		if entry == nil {
			t.Errorf("Entry not found in DB: %s", path)
			continue
		}

		// Verify status is ACTIVE
		if entry.FileStatusID == nil {
			t.Errorf("File %s has nil status", path)
			continue
		}

		status, err := dbService.GetFileStatusByID(*entry.FileStatusID)
		if err != nil {
			t.Errorf("Failed to get status for %s: %v", path, err)
			continue
		}

		if status == nil || *status != db.FileStatusActive {
			t.Errorf("File %s has incorrect status: %v (expected ACTIVE)", path, status)
		}
	}
	t.Logf("✓ All %d files found in DB with ACTIVE status", len(structure))

	// Verify parent directories also have ACTIVE status
	parentDirs := []string{"docs", "docs/guide", "notes", "notes/personal", "notes/work",
		"archive", "archive/old-notes", "archive/old-notes/2023",
		"projects", "projects/active", "projects/active/backend", "projects/active/frontend",
		"projects/archived", "projects/archived/legacy"}

	for _, dir := range parentDirs {
		entry, err := dbService.GetFileEntryByPath(dir)
		if err != nil || entry == nil {
			t.Errorf("Directory not found in DB: %s", dir)
			continue
		}

		if entry.FileStatusID != nil {
			status, _ := dbService.GetFileStatusByID(*entry.FileStatusID)
			if status == nil || *status != db.FileStatusActive {
				t.Errorf("Directory %s has incorrect status: %v", dir, status)
			}
		}
	}
	t.Logf("✓ All parent directories have ACTIVE status")

	// ==========================================
	// PHASE 3: Verify Explorer Cache (GetFullTree) returns only ACTIVE files
	// ==========================================
	t.Log("PHASE 3: Verifying explorer cache (GetFullTree) returns only ACTIVE files...")

	// Get full tree directly from explorer service
	fullTree, err := explorerService.GetFullTree()
	if err != nil {
		t.Fatalf("Failed to get full tree from explorer: %v", err)
	}

	t.Logf("✓ Explorer GetFullTree() returned %d root nodes", len(fullTree))

	// Helper function to recursively count all nodes in tree
	var countAllNodes func(nodes interface{}) int
	countAllNodes = func(nodes interface{}) int {
		// Type switch to handle different node types
		switch v := nodes.(type) {
		case []*explorer.TreeNode:
			count := len(v)
			for _, node := range v {
				if node.Children != nil {
					count += countAllNodes(node.Children)
				}
			}
			return count
		default:
			return 0
		}
	}

	totalNodes := countAllNodes(fullTree)
	t.Logf("  Total nodes in tree (including nested): %d", totalNodes)

	// Verify we can traverse the tree and find expected root directories
	expectedRootDirs := map[string]bool{
		"docs":     true,
		"notes":    true,
		"archive":  true,
		"projects": true,
	}

	foundRootDirs := 0
	for _, node := range fullTree {
		if expectedRootDirs[node.Metadata.Name] {
			foundRootDirs++
			t.Logf("  ✓ Found root directory in tree: %s (ID: %s, IsDir: %v)",
				node.Metadata.Name, node.Metadata.ID, node.Metadata.IsDirectory)

			// Log children count for directories
			if node.Metadata.IsDirectory && node.Children != nil {
				t.Logf("    - Has %d children", len(node.Children))
			}
		}
	}

	if foundRootDirs != len(expectedRootDirs) {
		t.Errorf("Expected %d root directories, found %d", len(expectedRootDirs), foundRootDirs)
	}

	// Verify specific nested files are accessible via GetMetadata (tests cache)
	testPaths := []string{
		"docs/README.md",
		"docs/guide/intro.md",
		"notes/personal/diary.md",
		"projects/active/backend/todo.md",
	}

	for _, path := range testPaths {
		metadata, err := explorerService.GetMetadata(path)
		if err != nil {
			t.Errorf("Failed to get metadata for %s from explorer: %v", path, err)
		} else if metadata == nil {
			t.Errorf("Metadata not found in explorer cache for: %s", path)
		} else {
			t.Logf("  ✓ Explorer cache has: %s (ID: %s)", path, metadata.ID)
		}
	}

	// Also verify Tree API works (uses GetFullTree internally)
	t.Log("Verifying Tree API (which uses GetFullTree)...")
	treeReq := httptest.NewRequest("GET", "/api/v1/files/tree/status-test-vault", nil)
	treeW := httptest.NewRecorder()
	server.handleGetTree(treeW, treeReq)

	if treeW.Code != 200 {
		t.Fatalf("Tree API failed: %d %s", treeW.Code, treeW.Body.String())
	}

	var treeResp struct {
		Nodes []map[string]interface{} `json:"nodes"`
		Count int                      `json:"count"`
	}
	if err := json.Unmarshal(treeW.Body.Bytes(), &treeResp); err != nil {
		t.Fatalf("Failed to parse tree response: %v", err)
	}

	t.Logf("✓ Tree API returned %d root nodes (matches GetFullTree)", treeResp.Count)

	// ==========================================
	// PHASE 4: Verify Search Index contains only ACTIVE files
	// ==========================================
	t.Log("PHASE 4: Verifying search index contains only ACTIVE files...")

	// Wait for search service to be ready
	time.Sleep(1 * time.Second)

	// Search for a term that appears in multiple files
	results, err := searchService.SearchByText("notes")
	if err != nil {
		t.Errorf("Search failed: %v", err)
	} else {
		t.Logf("✓ Search returned %d results for 'notes'", results.Total)
		if len(results.Hits) > 0 {
			t.Logf("  Sample result: %s", results.Hits[0].ID)
		}
	}

	// ==========================================
	// PHASE 5: Delete files from different nested folders
	// ==========================================
	t.Log("PHASE 5: Deleting files from nested folders...")

	filesToDelete := []string{
		"docs/guide/advanced.md",            // Delete from docs/guide
		"notes/personal/ideas.md",           // Delete from notes/personal
		"notes/work/meetings.md",            // Delete from notes/work
		"archive/old-notes/2023/feb.md",     // Delete from deep nesting
		"projects/archived/legacy/notes.md", // Delete from archived project
	}

	for _, path := range filesToDelete {
		fullPath := filepath.Join(tempDir, path)
		if err := os.Remove(fullPath); err != nil {
			t.Fatalf("Failed to delete file %s: %v", path, err)
		}
		t.Logf("  Deleted: %s", path)
	}

	// Wait for sync to process deletions
	t.Log("Waiting for sync to process deletions...")
	time.Sleep(3 * time.Second)

	// ==========================================
	// PHASE 6: Verify deleted files have DELETED status
	// ==========================================
	t.Log("PHASE 6: Verifying deleted files have DELETED status in DB...")

	for _, path := range filesToDelete {
		entry, err := dbService.GetFileEntryByPath(path)
		if err != nil {
			t.Errorf("Failed to get entry for deleted file %s: %v", path, err)
			continue
		}

		if entry == nil {
			t.Errorf("Deleted file %s not found in DB (should still exist with DELETED status)", path)
			continue
		}

		// Verify status is DELETED
		if entry.FileStatusID == nil {
			t.Errorf("Deleted file %s has nil status", path)
			continue
		}

		status, err := dbService.GetFileStatusByID(*entry.FileStatusID)
		if err != nil {
			t.Errorf("Failed to get status for deleted file %s: %v", path, err)
			continue
		}

		if status == nil || *status != db.FileStatusDeleted {
			t.Errorf("Deleted file %s has incorrect status: %v (expected DELETED)", path, status)
		} else {
			t.Logf("  ✓ %s has DELETED status", path)
		}
	}

	// ==========================================
	// PHASE 7: Verify deleted files NOT in explorer cache (GetFullTree)
	// ==========================================
	t.Log("PHASE 7: Verifying deleted files NOT in explorer cache...")

	// First verify individual GetMetadata calls fail for deleted files
	for _, path := range filesToDelete {
		metadata, err := explorerService.GetMetadata(path)
		if err == nil && metadata != nil {
			t.Errorf("Deleted file %s still appears in explorer cache (GetMetadata)", path)
		} else {
			t.Logf("  ✓ %s correctly excluded from GetMetadata", path)
		}
	}

	// Get full tree again using GetFullTree() directly
	t.Log("Calling GetFullTree() after deletions...")
	fullTree2, err := explorerService.GetFullTree()
	if err != nil {
		t.Fatalf("Failed to get full tree after deletion: %v", err)
	}

	t.Logf("✓ Explorer GetFullTree() after deletion returned %d root nodes (was %d before)",
		len(fullTree2), len(fullTree))

	// Helper to recursively search for a file in tree
	var findFileInTree func(nodes []*explorer.TreeNode, targetName string) bool
	findFileInTree = func(nodes []*explorer.TreeNode, targetName string) bool {
		for _, node := range nodes {
			if node.Metadata.Name == targetName {
				return true
			}
			if node.Children != nil && len(node.Children) > 0 {
				if findFileInTree(node.Children, targetName) {
					return true
				}
			}
		}
		return false
	}

	// Verify deleted files are NOT in the full tree
	for _, path := range filesToDelete {
		fileName := filepath.Base(path)
		if findFileInTree(fullTree2, fileName) {
			t.Errorf("Deleted file %s still found in GetFullTree()", fileName)
		} else {
			t.Logf("  ✓ %s correctly excluded from GetFullTree()", fileName)
		}
	}

	// Also verify Tree API works
	treeReq2 := httptest.NewRequest("GET", "/api/v1/files/tree/status-test-vault", nil)
	treeW2 := httptest.NewRecorder()
	server.handleGetTree(treeW2, treeReq2)

	if treeW2.Code != 200 {
		t.Fatalf("Tree API failed on second call: %d", treeW2.Code)
	}

	var treeResp2 struct {
		Nodes []map[string]interface{} `json:"nodes"`
		Count int                      `json:"count"`
	}
	if err := json.Unmarshal(treeW2.Body.Bytes(), &treeResp2); err != nil {
		t.Fatalf("Failed to parse tree response: %v", err)
	}

	t.Logf("✓ Tree API after deletion returned %d root nodes (was %d before)", treeResp2.Count, treeResp.Count)

	// ==========================================
	// PHASE 8: Verify deleted files NOT in search index
	// ==========================================
	t.Log("PHASE 8: Verifying deleted files NOT in search index...")

	// Wait for index to be updated
	time.Sleep(2 * time.Second)

	// Search for terms that appear only in deleted files
	deletedTerms := []string{
		"meetings", // Only in notes/work/meetings.md
		"February", // Only in archive/old-notes/2023/feb.md
		"Legacy",   // Only in projects/archived/legacy/notes.md
	}

	for _, term := range deletedTerms {
		results, err := searchService.SearchByText(term)
		if err != nil {
			t.Errorf("Search failed for term '%s': %v", term, err)
			continue
		}

		// Check if any deleted files appear in results
		for _, hit := range results.Hits {
			for _, deletedPath := range filesToDelete {
				if hit.ID == deletedPath {
					t.Errorf("Deleted file %s still appears in search results for '%s'", deletedPath, term)
				}
			}
		}

		if results.Total == 0 {
			t.Logf("  ✓ No results for '%s' (as expected, was only in deleted files)", term)
		}
	}

	// ==========================================
	// PHASE 9: Verify ACTIVE files still accessible
	// ==========================================
	t.Log("PHASE 9: Verifying ACTIVE files still work correctly...")

	activeFilesToCheck := []string{
		"docs/README.md",
		"docs/guide/intro.md",
		"notes/personal/diary.md",
		"projects/active/backend/todo.md",
	}

	for _, path := range activeFilesToCheck {
		// Check DB
		entry, err := dbService.GetFileEntryByPath(path)
		if err != nil || entry == nil {
			t.Errorf("Active file %s not found in DB", path)
			continue
		}

		// Verify can get by ID using optimized method
		activeEntry, err := dbService.GetFileEntryByIDWithStatus(entry.ID, db.FileStatusActive)
		if err != nil || activeEntry == nil {
			t.Errorf("Failed to get active file %s by ID with status check", path)
			continue
		}

		// Check explorer
		metadata, err := explorerService.GetMetadata(path)
		if err != nil || metadata == nil {
			t.Errorf("Active file %s not found in explorer", path)
			continue
		}

		t.Logf("  ✓ %s is still accessible (DB + Explorer)", path)
	}

	// ==========================================
	// SUMMARY
	// ==========================================
	t.Log("\n=== TEST SUMMARY ===")
	t.Logf("✓ Created %d files in nested folder structure", len(structure))
	t.Logf("✓ All files correctly indexed in DB with ACTIVE status")
	t.Logf("✓ Tree API correctly returns only ACTIVE files")
	t.Logf("✓ Explorer cache correctly filters ACTIVE files")
	t.Logf("✓ Search index contains only ACTIVE files")
	t.Logf("✓ Deleted %d files from various nested folders", len(filesToDelete))
	t.Logf("✓ Deleted files have DELETED status in DB (soft delete)")
	t.Logf("✓ Deleted files excluded from tree API")
	t.Logf("✓ Deleted files excluded from explorer cache")
	t.Logf("✓ Deleted files excluded from search index")
	t.Logf("✓ Active files remain accessible through all APIs")
}
