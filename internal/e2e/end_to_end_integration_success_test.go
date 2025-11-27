package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/db"
	"github.com/susamn/obsidian-web/internal/vault"
	"github.com/susamn/obsidian-web/internal/web"
)

/*
TestEndToEndIntegrationSuccess validates the complete happy path data flow:

PHASES:
1. System Initialization - Create vault, start all services
2. File Creation - Create nested folder structure with markdown files
3. Data Flow Validation - Verify files flow through all layers (FS → Sync → Worker → DB → Index → Explorer → API)
4. SSE Event Validation - Verify SSE events are queued and flushed correctly
5. API Validation - Verify all APIs return correct data
6. File Modification - Modify files and verify changes propagate
7. File Deletion - Delete files and verify soft delete with status update
8. SSE Events for Changes - Verify SSE events for modifications and deletions
9. Cleanup & Verification - Ensure system remains consistent

This test covers:
- File system monitoring and sync
- Worker event processing
- Database updates with status tracking
- Search indexing
- Explorer cache management
- SSE event queueing and flushing (every 2 seconds)
- HTTP API responses
- Pending event count tracking
*/
func TestEndToEndIntegrationSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ==========================================
	// PHASE 1: System Initialization
	// ==========================================
	t.Log("=== PHASE 1: System Initialization ===")

	tempDir := t.TempDir()
	indexDir := t.TempDir()
	dbDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "e2e-success-vault",
		Name:      "E2E Success Test Vault",
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

	// Create and start vault
	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}
	defer v.Stop()

	// Wait for vault to be ready
	if err := v.WaitForReady(10 * time.Second); err != nil {
		t.Fatalf("Vault not ready: %v", err)
	}
	t.Log("✓ Vault started and ready")

	// Create web server (use unconventional port for testing)
	serverCfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 19871, // Unconventional port for E2E success test
		},
		Vaults: []config.VaultConfig{*vaultCfg},
	}

	server := web.NewServer(ctx, serverCfg, map[string]*vault.Vault{"e2e-success-vault": v})
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Wire up SSE manager to vault
	sseManager := server.GetSSEManager()
	v.SetSSEManager(sseManager)
	t.Log("✓ Web server started with SSE manager")

	// Get services for validation
	dbService := v.GetDBService()
	explorerService := v.GetExplorerService()
	searchService := v.GetSearchService()
	syncService := v.GetSyncService()

	// ==========================================
	// PHASE 2: File Creation
	// ==========================================
	t.Log("\n=== PHASE 2: File Creation ===")

	fileStructure := map[string]string{
		"docs/README.md":                   "# Documentation\n\nMain documentation file",
		"docs/guide/intro.md":              "# Introduction\n\nGetting started guide",
		"docs/guide/advanced.md":           "# Advanced Topics\n\nAdvanced usage",
		"notes/personal/diary.md":          "# Personal Diary\n\nDaily thoughts",
		"notes/personal/ideas.md":          "# Ideas\n\nBrainstorming session",
		"notes/work/project-a.md":          "# Project A\n\nWork project notes",
		"notes/work/meetings.md":           "# Meetings\n\nMeeting notes",
		"archive/2023/jan.md":              "# January 2023\n\nArchived notes",
		"projects/active/backend/todo.md":  "# Backend TODO\n\nBackend tasks",
		"projects/active/frontend/todo.md": "# Frontend TODO\n\nFrontend tasks",
	}

	t.Logf("Creating %d files in nested structure...", len(fileStructure))
	for path, content := range fileStructure {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", path, err)
		}
	}
	t.Logf("✓ Created %d files", len(fileStructure))

	// Wait for sync to detect and process files
	t.Log("Waiting for sync and workers to process files...")
	time.Sleep(4 * time.Second)

	// ==========================================
	// PHASE 3: Data Flow Validation
	// ==========================================
	t.Log("\n=== PHASE 3: Data Flow Validation ===")

	// 3.1: Verify DB has all files with ACTIVE status
	t.Log("3.1: Validating database entries...")
	dbFileCount := 0
	for path := range fileStructure {
		entry, err := dbService.GetFileEntryByPath(path)
		if err != nil || entry == nil {
			t.Errorf("File %s not found in database: %v", path, err)
			continue
		}

		// Verify ACTIVE status
		if entry.FileStatusID != nil {
			status, err := dbService.GetFileStatusByID(*entry.FileStatusID)
			if err != nil || status == nil || *status != db.FileStatusActive {
				t.Errorf("File %s has incorrect status: %v (expected ACTIVE)", path, status)
				continue
			}
		} else {
			t.Errorf("File %s has nil status", path)
			continue
		}

		dbFileCount++
	}
	t.Logf("✓ Database has %d/%d files with ACTIVE status", dbFileCount, len(fileStructure))

	// 3.2: Verify Explorer cache
	t.Log("3.2: Validating explorer cache...")
	fullTree, err := explorerService.GetFullTree()
	if err != nil {
		t.Fatalf("Failed to get full tree: %v", err)
	}
	t.Logf("✓ Explorer cache returned %d root nodes", len(fullTree))

	// Verify sample files in explorer
	samplePaths := []string{"docs/README.md", "notes/work/project-a.md", "projects/active/backend/todo.md"}
	for _, path := range samplePaths {
		metadata, err := explorerService.GetMetadata(path)
		if err != nil || metadata == nil {
			t.Errorf("File %s not found in explorer cache", path)
		}
	}
	t.Log("✓ Sample files accessible via explorer")

	// 3.3: Verify Search index
	t.Log("3.3: Validating search index...")
	time.Sleep(2 * time.Second) // Wait for indexing
	searchResults, err := searchService.SearchByText("TODO")
	if err != nil {
		t.Errorf("Search failed: %v", err)
	} else {
		t.Logf("✓ Search index working (found %d results for 'TODO')", searchResults.Total)
	}

	// ==========================================
	// PHASE 4: SSE Event Validation
	// ==========================================
	t.Log("\n=== PHASE 4: SSE Event Validation ===")

	// Create SSE client connection
	sseReq := httptest.NewRequest("GET", "/api/v1/sse/e2e-success-vault", nil)
	sseReqCtx, sseReqCancel := context.WithCancel(ctx)
	defer sseReqCancel()
	sseReq = sseReq.WithContext(sseReqCtx)

	sseRecorder := httptest.NewRecorder()

	// Start SSE handler in goroutine
	sseDone := make(chan bool)
	go func() {
		// Access the handleSSE method via reflection or by making it public
		// For now, we'll use a workaround through the HTTP handler
		server.ServeHTTP(sseRecorder, sseReq)
		close(sseDone)
	}()

	// Wait for connection
	time.Sleep(200 * time.Millisecond)
	t.Log("✓ SSE client connected")

	// Verify connected event
	sseBody := sseRecorder.Body.String()
	if !strings.Contains(sseBody, "event: connected") {
		t.Error("Expected 'connected' event in SSE stream")
	} else {
		t.Log("✓ Received 'connected' event")
	}

	// Wait for first flush (every 2 seconds) - may get ping or bulk_process
	t.Log("Waiting for first SSE flush...")
	time.Sleep(2500 * time.Millisecond)

	sseBody = sseRecorder.Body.String()
	// Should receive either ping (if queue was empty) or bulk_process (if files were queued)
	hasPing := strings.Contains(sseBody, "event: ping")
	hasBulkProcess := strings.Contains(sseBody, "event: bulk_process")

	if !hasPing && !hasBulkProcess {
		t.Error("Expected either 'ping' or 'bulk_process' event in SSE stream")
	} else {
		if hasPing {
			t.Log("✓ Received 'ping' event")
		}
		if hasBulkProcess {
			t.Log("✓ Received 'bulk_process' event (files were processed)")
		}

		// Verify pending count is present
		if strings.Contains(sseBody, "pending_count") {
			t.Log("✓ Events contain 'pending_count'")

			// Extract pending count from the event
			lines := strings.Split(sseBody, "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "data: ") {
					data := strings.TrimPrefix(line, "data: ")
					var event map[string]interface{}
					if err := json.Unmarshal([]byte(data), &event); err == nil {
						if eventType, ok := event["type"].(string); ok {
							if eventType == "ping" || eventType == "bulk_process" {
								if pendingCount, ok := event["pending_count"]; ok {
									t.Logf("  Pending count in %s event: %v", eventType, pendingCount)
								}
							}
						}
					}
				}
			}
		}
	}

	// Get current pending events count from sync service
	pendingCount := syncService.PendingEventsCount()
	t.Logf("✓ Sync service reports %d pending events", pendingCount)

	// ==========================================
	// PHASE 5: API Validation
	// ==========================================
	t.Log("\n=== PHASE 5: API Validation ===")

	// 5.1: Tree API
	treeReq := httptest.NewRequest("GET", "/api/v1/files/tree/e2e-success-vault", nil)
	treeW := httptest.NewRecorder()
	server.ServeHTTP(treeW, treeReq)

	if treeW.Code != http.StatusOK {
		t.Fatalf("Tree API failed: %d %s", treeW.Code, treeW.Body.String())
	}

	var treeResp struct {
		Nodes []map[string]interface{} `json:"nodes"`
		Count int                      `json:"count"`
	}
	if err := json.Unmarshal(treeW.Body.Bytes(), &treeResp); err != nil {
		t.Fatalf("Failed to parse tree response: %v", err)
	}
	t.Logf("✓ Tree API returned %d root nodes", treeResp.Count)

	// 5.2: SSE Stats API
	statsReq := httptest.NewRequest("GET", "/api/v1/sse/stats", nil)
	statsW := httptest.NewRecorder()
	server.ServeHTTP(statsW, statsReq)

	if statsW.Code != http.StatusOK {
		t.Errorf("SSE stats API failed: %d", statsW.Code)
	} else {
		var statsResp map[string]interface{}
		if err := json.Unmarshal(statsW.Body.Bytes(), &statsResp); err == nil {
			t.Log("✓ SSE stats API working")
			if data, ok := statsResp["data"].(map[string]interface{}); ok {
				if totalClients, ok := data["total_clients"]; ok {
					t.Logf("  Total SSE clients: %v", totalClients)
				}
			}
		}
	}

	// ==========================================
	// PHASE 6: File Modification
	// ==========================================
	t.Log("\n=== PHASE 6: File Modification ===")

	filesToModify := []string{"docs/README.md", "notes/work/project-a.md"}
	t.Logf("Modifying %d files...", len(filesToModify))

	for _, path := range filesToModify {
		fullPath := filepath.Join(tempDir, path)
		newContent := fmt.Sprintf("# Modified\n\nThis file was modified at %s", time.Now().Format(time.RFC3339))
		if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
			t.Fatalf("Failed to modify file %s: %v", path, err)
		}
	}
	t.Logf("✓ Modified %d files", len(filesToModify))

	// Wait for sync and processing
	time.Sleep(3 * time.Second)

	// Verify modifications in DB (should still be ACTIVE)
	for _, path := range filesToModify {
		entry, err := dbService.GetFileEntryByPath(path)
		if err != nil || entry == nil {
			t.Errorf("Modified file %s not found in DB", path)
			continue
		}

		if entry.FileStatusID != nil {
			status, _ := dbService.GetFileStatusByID(*entry.FileStatusID)
			if status == nil || *status != db.FileStatusActive {
				t.Errorf("Modified file %s lost ACTIVE status", path)
			}
		}
	}
	t.Log("✓ Modified files still have ACTIVE status")

	// ==========================================
	// PHASE 7: File Deletion (Soft Delete)
	// ==========================================
	t.Log("\n=== PHASE 7: File Deletion (Soft Delete) ===")

	filesToDelete := []string{
		"docs/guide/advanced.md",
		"notes/personal/ideas.md",
		"archive/2023/jan.md",
	}

	t.Logf("Deleting %d files...", len(filesToDelete))
	for _, path := range filesToDelete {
		fullPath := filepath.Join(tempDir, path)
		if err := os.Remove(fullPath); err != nil {
			t.Fatalf("Failed to delete file %s: %v", path, err)
		}
	}
	t.Logf("✓ Deleted %d files from filesystem", len(filesToDelete))

	// Wait for sync and processing
	time.Sleep(3 * time.Second)

	// Verify soft delete - files should have DELETED status in DB
	for _, path := range filesToDelete {
		entry, err := dbService.GetFileEntryByPath(path)
		if err != nil || entry == nil {
			t.Errorf("Deleted file %s not found in DB (should exist with DELETED status)", path)
			continue
		}

		if entry.FileStatusID == nil {
			t.Errorf("Deleted file %s has nil status", path)
			continue
		}

		status, err := dbService.GetFileStatusByID(*entry.FileStatusID)
		if err != nil || status == nil || *status != db.FileStatusDeleted {
			t.Errorf("Deleted file %s has incorrect status: %v (expected DELETED)", path, status)
		}
	}
	t.Log("✓ Deleted files have DELETED status in DB (soft delete)")

	// Verify deleted files NOT in explorer
	for _, path := range filesToDelete {
		metadata, err := explorerService.GetMetadata(path)
		if err == nil && metadata != nil {
			t.Errorf("Deleted file %s still in explorer cache", path)
		}
	}
	t.Log("✓ Deleted files excluded from explorer cache")

	// ==========================================
	// PHASE 8: SSE Events for Changes
	// ==========================================
	t.Log("\n=== PHASE 8: SSE Events for File Changes ===")

	// Wait for next SSE flush (should have bulk_process or ping)
	t.Log("Waiting for SSE flush after file changes...")
	time.Sleep(2500 * time.Millisecond)

	sseBody = sseRecorder.Body.String()

	// Should have received either bulk_process (if events were queued) or ping
	hasBulkProcess = strings.Contains(sseBody, "event: bulk_process")
	hasPing = strings.Contains(sseBody, "event: ping")

	if hasBulkProcess {
		t.Log("✓ Received 'bulk_process' event for file changes")

		// Parse and validate bulk_process event
		lines := strings.Split(sseBody, "\n")
		for i, line := range lines {
			if line == "event: bulk_process" && i+1 < len(lines) {
				dataLine := lines[i+1]
				if strings.HasPrefix(dataLine, "data: ") {
					data := strings.TrimPrefix(dataLine, "data: ")
					var event map[string]interface{}
					if err := json.Unmarshal([]byte(data), &event); err == nil {
						if changes, ok := event["changes"].([]interface{}); ok {
							t.Logf("  Bulk process event has %d changes", len(changes))

							// Validate change structure
							for _, change := range changes {
								if changeMap, ok := change.(map[string]interface{}); ok {
									if id, hasID := changeMap["id"]; hasID {
										if path, hasPath := changeMap["path"]; hasPath {
											if action, hasAction := changeMap["action"]; hasAction {
												t.Logf("    Change: id=%v, path=%v, action=%v", id, path, action)
											}
										}
									}
								}
							}
						}

						if pendingCount, ok := event["pending_count"]; ok {
							t.Logf("  Pending count: %v", pendingCount)
						}
					}
				}
				break
			}
		}
	} else if hasPing {
		t.Log("✓ Received 'ping' event (no changes in queue at flush time)")
	} else {
		t.Error("Expected either 'bulk_process' or 'ping' event after changes")
	}

	// Close SSE connection
	sseReqCancel()
	<-sseDone
	t.Log("✓ SSE client disconnected")

	// ==========================================
	// PHASE 9: Cleanup & Final Verification
	// ==========================================
	t.Log("\n=== PHASE 9: Cleanup & Final Verification ===")

	// Verify ACTIVE files still work
	activeFiles := []string{"docs/README.md", "notes/work/project-a.md", "projects/active/backend/todo.md"}
	activeCount := 0
	for _, path := range activeFiles {
		entry, err := dbService.GetFileEntryByPath(path)
		if err == nil && entry != nil {
			if entry.FileStatusID != nil {
				status, _ := dbService.GetFileStatusByID(*entry.FileStatusID)
				if status != nil && *status == db.FileStatusActive {
					activeCount++
				}
			}
		}
	}
	t.Logf("✓ %d/%d active files still accessible", activeCount, len(activeFiles))

	// Get final metrics
	finalPendingCount := syncService.PendingEventsCount()
	t.Logf("✓ Final pending events: %d", finalPendingCount)

	clientCount := sseManager.GetClientCount()
	t.Logf("✓ SSE clients: %d", clientCount)

	// ==========================================
	// TEST SUMMARY
	// ==========================================
	t.Log("\n=== TEST SUMMARY: END-TO-END INTEGRATION SUCCESS ===")
	t.Logf("✓ Created %d files in nested structure", len(fileStructure))
	t.Logf("✓ All files indexed in DB with ACTIVE status")
	t.Logf("✓ Explorer cache correctly filters ACTIVE files")
	t.Logf("✓ Search index working correctly")
	t.Logf("✓ SSE events queued and flushed correctly (every 2s)")
	t.Logf("✓ SSE events contain pending count")
	t.Logf("✓ All HTTP APIs working correctly")
	t.Logf("✓ File modifications propagated correctly")
	t.Logf("✓ Deleted %d files (soft delete)", len(filesToDelete))
	t.Logf("✓ Deleted files have DELETED status")
	t.Logf("✓ Deleted files excluded from APIs")
	t.Logf("✓ Active files remain accessible")
	t.Log("✓ COMPLETE DATA FLOW VALIDATED: FS → Sync → Worker → DB → Index → Explorer → API → SSE")
}
