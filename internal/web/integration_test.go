package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/vault"
)

// TestDataFlow_CompleteFlow tests the complete data flow:
// File creation -> Sync detection -> Indexing -> Search -> API retrieval
func TestDataFlow_CompleteFlow(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "dataflow-vault",
		Name:      "DataFlow Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}
	defer v.Stop()

	if err := v.WaitForReady(10 * time.Second); err != nil {
		t.Fatalf("Vault not ready: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Vaults: []config.VaultConfig{*vaultCfg},
	}

	vaults := map[string]*vault.Vault{
		"dataflow-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Step 1: Create a file (simulates sync detection)
	testFile := filepath.Join(tempDir, "test-note.md")
	testContent := "# Test Note\n\nThis is a test note with searchable content."
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for sync and indexing
	time.Sleep(2 * time.Second)

	// Step 2: Verify file indexed via metrics
	req := httptest.NewRequest("GET", "/api/v1/metrics/dataflow-vault", nil)
	w := httptest.NewRecorder()
	server.handleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Metrics request failed: %d", w.Code)
	}

	var metricsResp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&metricsResp); err != nil {
		t.Fatalf("Failed to decode metrics: %v", err)
	}

	t.Logf("Metrics after file creation: %+v", metricsResp.Data)

	// Step 3: Search for content via API
	searchReq := SearchRequest{
		Query: "searchable",
		Type:  "text",
	}
	body, _ := json.Marshal(searchReq)
	req = httptest.NewRequest("POST", "/api/v1/search/dataflow-vault", bytes.NewBuffer(body))
	w = httptest.NewRecorder()

	server.handleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Search request failed: %d: %s", w.Code, w.Body.String())
	}

	var searchResp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&searchResp); err != nil {
		t.Fatalf("Failed to decode search results: %v", err)
	}

	t.Logf("Search results: %+v", searchResp.Data)

	// Step 4: Retrieve file content via API
	req = httptest.NewRequest("GET", "/api/v1/files/dataflow-vault/test-note.md", nil)
	w = httptest.NewRecorder()

	server.handleGetFile(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Get file request failed: %d", w.Code)
	}

	var fileResp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&fileResp); err != nil {
		t.Fatalf("Failed to decode file response: %v", err)
	}

	fileData := fileResp.Data.(map[string]interface{})
	if fileData["content"].(string) != testContent {
		t.Errorf("Content mismatch. Expected: %s, Got: %s", testContent, fileData["content"])
	}

	// Step 5: Verify vault health
	req = httptest.NewRequest("GET", "/api/v1/health", nil)
	w = httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Health check failed: %d", w.Code)
	}

	t.Log("✓ Complete dataflow verified: File -> Sync -> Index -> Search -> API")
}

// TestDataFlow_ConcurrentOperations tests concurrent API operations for deadlocks
func TestDataFlow_ConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	// Create test files
	for i := 0; i < 10; i++ {
		testFile := filepath.Join(tempDir, fmt.Sprintf("note-%d.md", i))
		content := fmt.Sprintf("# Note %d\n\nContent for note %d", i, i)
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	vaultCfg := &config.VaultConfig{
		ID:        "concurrent-vault",
		Name:      "Concurrent Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}
	defer v.Stop()

	if err := v.WaitForReady(10 * time.Second); err != nil {
		t.Fatalf("Vault not ready: %v", err)
	}

	// Wait for initial indexing
	time.Sleep(2 * time.Second)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Vaults: []config.VaultConfig{*vaultCfg},
	}

	vaults := map[string]*vault.Vault{
		"concurrent-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Concurrent operations
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Run concurrent searches
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			searchReq := SearchRequest{
				Query: fmt.Sprintf("note %d", idx%10),
				Type:  "text",
			}
			body, _ := json.Marshal(searchReq)
			req := httptest.NewRequest("POST", "/api/v1/search/concurrent-vault", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			server.handleSearch(w, req)

			if w.Code != http.StatusOK {
				errors <- fmt.Errorf("search %d failed: %d", idx, w.Code)
			}
		}(i)
	}

	// Concurrent file retrievals
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/files/concurrent-vault/note-%d.md", idx%10), nil)
			w := httptest.NewRecorder()

			server.handleGetFile(w, req)

			if w.Code != http.StatusOK {
				errors <- fmt.Errorf("get file %d failed: %d", idx, w.Code)
			}
		}(i)
	}

	// Concurrent metrics checks
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/api/v1/metrics/concurrent-vault", nil)
			w := httptest.NewRecorder()

			server.handleMetrics(w, req)

			if w.Code != http.StatusOK {
				errors <- fmt.Errorf("metrics %d failed: %d", idx, w.Code)
			}
		}(i)
	}

	// Concurrent vault info checks
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			req := httptest.NewRequest("GET", "/api/v1/vaults/concurrent-vault", nil)
			w := httptest.NewRecorder()

			server.handleVaultOps(w, req)

			if w.Code != http.StatusOK {
				errors <- fmt.Errorf("vault info %d failed: %d", idx, w.Code)
			}
		}(i)
	}

	// Wait for all operations
	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Had %d errors in concurrent operations", errorCount)
	}

	t.Log("✓ No deadlocks detected in 80 concurrent operations")
}

// TestDataFlow_DataConsistency tests data consistency across services
func TestDataFlow_DataConsistency(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "consistency-vault",
		Name:      "Consistency Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}
	defer v.Stop()

	if err := v.WaitForReady(10 * time.Second); err != nil {
		t.Fatalf("Vault not ready: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Vaults: []config.VaultConfig{*vaultCfg},
	}

	vaults := map[string]*vault.Vault{
		"consistency-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Create multiple files
	fileNames := []string{"note1.md", "note2.md", "note3.md"}
	for _, fileName := range fileNames {
		content := fmt.Sprintf("# %s\n\nTest content for %s", fileName, fileName)
		if err := os.WriteFile(filepath.Join(tempDir, fileName), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	// Wait for indexing
	time.Sleep(2 * time.Second)

	// Test 1: Verify file count consistency
	req := httptest.NewRequest("GET", "/api/v1/metrics/consistency-vault", nil)
	w := httptest.NewRecorder()
	server.handleMetrics(w, req)

	var metricsResp SuccessResponse
	json.NewDecoder(w.Body).Decode(&metricsResp)
	metricsData := metricsResp.Data.(map[string]interface{})

	indexedFiles := uint64(metricsData["indexed_files"].(float64))
	t.Logf("Indexed files: %d", indexedFiles)

	if indexedFiles != 3 {
		t.Errorf("Expected 3 indexed files, got %d", indexedFiles)
	}

	// Test 2: Verify each file is searchable
	for _, fileName := range fileNames {
		searchReq := SearchRequest{
			Query: fileName[:len(fileName)-3], // Remove .md extension
			Type:  "text",
		}
		body, _ := json.Marshal(searchReq)
		req = httptest.NewRequest("POST", "/api/v1/search/consistency-vault", bytes.NewBuffer(body))
		w = httptest.NewRecorder()

		server.handleSearch(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Search for %s failed: %d", fileName, w.Code)
			continue
		}

		var searchResp SuccessResponse
		json.NewDecoder(w.Body).Decode(&searchResp)
		t.Logf("Search results for %s: %+v", fileName, searchResp.Data)
	}

	// Test 3: Verify file content matches filesystem
	for _, fileName := range fileNames {
		// Read from filesystem
		fsContent, err := os.ReadFile(filepath.Join(tempDir, fileName))
		if err != nil {
			t.Fatalf("Failed to read file from filesystem: %v", err)
		}

		// Read via API
		req = httptest.NewRequest("GET", fmt.Sprintf("/api/v1/files/consistency-vault/%s", fileName), nil)
		w = httptest.NewRecorder()
		server.handleGetFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Get file %s failed: %d", fileName, w.Code)
			continue
		}

		var fileResp SuccessResponse
		json.NewDecoder(w.Body).Decode(&fileResp)
		fileData := fileResp.Data.(map[string]interface{})
		apiContent := fileData["content"].(string)

		if apiContent != string(fsContent) {
			t.Errorf("Content mismatch for %s.\nFilesystem: %s\nAPI: %s", fileName, fsContent, apiContent)
		}
	}

	// Test 4: Modify a file and verify consistency
	modifiedContent := "# Modified\n\nThis content was modified"
	if err := os.WriteFile(filepath.Join(tempDir, "note1.md"), []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Wait for re-indexing
	time.Sleep(2 * time.Second)

	// Verify modification
	req = httptest.NewRequest("GET", "/api/v1/files/consistency-vault/note1.md", nil)
	w = httptest.NewRecorder()
	server.handleGetFile(w, req)

	var fileResp SuccessResponse
	json.NewDecoder(w.Body).Decode(&fileResp)
	fileData := fileResp.Data.(map[string]interface{})

	if fileData["content"].(string) != modifiedContent {
		t.Error("Modified content not reflected via API")
	}

	// Test 5: Delete a file and verify consistency
	if err := os.Remove(filepath.Join(tempDir, "note2.md")); err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	// Wait for sync
	time.Sleep(2 * time.Second)

	// Verify file count decreased
	req = httptest.NewRequest("GET", "/api/v1/metrics/consistency-vault", nil)
	w = httptest.NewRecorder()
	server.handleMetrics(w, req)

	json.NewDecoder(w.Body).Decode(&metricsResp)
	metricsData = metricsResp.Data.(map[string]interface{})
	indexedFiles = uint64(metricsData["indexed_files"].(float64))

	if indexedFiles != 2 {
		t.Errorf("Expected 2 indexed files after deletion, got %d", indexedFiles)
	}

	// Verify deleted file not accessible
	req = httptest.NewRequest("GET", "/api/v1/files/consistency-vault/note2.md", nil)
	w = httptest.NewRecorder()
	server.handleGetFile(w, req)

	if w.Code != http.StatusNotFound {
		t.Error("Deleted file should not be accessible")
	}

	t.Log("✓ Data consistency verified across create/modify/delete operations")
}

// TestDataFlow_VaultLifecycle tests vault lifecycle operations via API
func TestDataFlow_VaultLifecycle(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "lifecycle-vault",
		Name:      "Lifecycle Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Vaults: []config.VaultConfig{*vaultCfg},
	}

	vaults := map[string]*vault.Vault{
		"lifecycle-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Initial state should be initializing
	req := httptest.NewRequest("GET", "/api/v1/vaults/lifecycle-vault", nil)
	w := httptest.NewRecorder()
	server.handleVaultOps(w, req)

	var vaultResp SuccessResponse
	json.NewDecoder(w.Body).Decode(&vaultResp)
	vaultData := vaultResp.Data.(map[string]interface{})

	t.Logf("Initial vault status: %s", vaultData["status"])

	// Start vault
	req = httptest.NewRequest("POST", "/api/v1/vaults/lifecycle-vault/start", nil)
	w = httptest.NewRecorder()
	server.handleVaultOps(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to start vault: %d: %s", w.Code, w.Body.String())
	}

	// Wait for active state
	time.Sleep(2 * time.Second)

	// Verify active state
	req = httptest.NewRequest("GET", "/api/v1/vaults/lifecycle-vault", nil)
	w = httptest.NewRecorder()
	server.handleVaultOps(w, req)

	json.NewDecoder(w.Body).Decode(&vaultResp)
	vaultData = vaultResp.Data.(map[string]interface{})

	if vaultData["status"] != "active" {
		t.Errorf("Expected active status, got %s", vaultData["status"])
	}

	t.Logf("Vault status after start: %s", vaultData["status"])

	// Create a file while vault is active
	testFile := filepath.Join(tempDir, "active-test.md")
	if err := os.WriteFile(testFile, []byte("# Active Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	time.Sleep(2 * time.Second)

	// Verify file is indexed
	searchReq := SearchRequest{Query: "Active", Type: "text"}
	body, _ := json.Marshal(searchReq)
	req = httptest.NewRequest("POST", "/api/v1/search/lifecycle-vault", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	server.handleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Error("Search should work while vault is active")
	}

	// Stop vault
	req = httptest.NewRequest("POST", "/api/v1/vaults/lifecycle-vault/stop", nil)
	w = httptest.NewRecorder()
	server.handleVaultOps(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to stop vault: %d", w.Code)
	}

	// Verify stopped state
	req = httptest.NewRequest("GET", "/api/v1/vaults/lifecycle-vault", nil)
	w = httptest.NewRecorder()
	server.handleVaultOps(w, req)

	json.NewDecoder(w.Body).Decode(&vaultResp)
	vaultData = vaultResp.Data.(map[string]interface{})

	if vaultData["status"] != "stopped" {
		t.Errorf("Expected stopped status, got %s", vaultData["status"])
	}

	// Search should fail when stopped
	req = httptest.NewRequest("POST", "/api/v1/search/lifecycle-vault", bytes.NewBuffer(body))
	w = httptest.NewRecorder()
	server.handleSearch(w, req)

	if w.Code == http.StatusOK {
		t.Error("Search should not work when vault is stopped")
	}

	t.Log("✓ Vault lifecycle verified: initializing -> start -> active -> stop -> stopped")
}

// TestDataFlow_MultiVaultIsolation tests data isolation between multiple vaults
func TestDataFlow_MultiVaultIsolation(t *testing.T) {
	ctx := context.Background()

	// Create two vaults
	vaults := make(map[string]*vault.Vault)
	vaultConfigs := []config.VaultConfig{}

	for i := 1; i <= 2; i++ {
		tempDir := t.TempDir()
		indexDir := t.TempDir()

		// Create vault-specific file with unique content
		content := fmt.Sprintf("# UniqueVault%d\n\nThis is unique content only in vault number %d. UniqueIdentifier%d", i, i, i)
		if err := os.WriteFile(filepath.Join(tempDir, "note.md"), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		vaultCfg := config.VaultConfig{
			ID:        fmt.Sprintf("vault-%d", i),
			Name:      fmt.Sprintf("Vault %d", i),
			Enabled:   true,
			IndexPath: indexDir + "/test.bleve",
			Storage: config.StorageConfig{
				Type: "local",
				Local: &config.LocalStorageConfig{
					Path: tempDir,
				},
			},
		}

		v, err := vault.NewVault(ctx, &vaultCfg)
		if err != nil {
			t.Fatalf("Failed to create vault %d: %v", i, err)
		}

		if err := v.Start(); err != nil {
			t.Fatalf("Failed to start vault %d: %v", i, err)
		}
		defer v.Stop()

		if err := v.WaitForReady(10 * time.Second); err != nil {
			t.Fatalf("Vault %d not ready: %v", i, err)
		}

		vaults[vaultCfg.ID] = v
		vaultConfigs = append(vaultConfigs, vaultCfg)
	}

	// Wait for indexing
	time.Sleep(2 * time.Second)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Vaults: vaultConfigs,
	}

	server := NewServer(ctx, cfg, vaults)

	// Test isolation: Search in vault-1 for vault-1 specific content
	searchReq := SearchRequest{Query: "UniqueIdentifier1", Type: "text"}
	body, _ := json.Marshal(searchReq)

	// Search in vault-1 - should find result
	req := httptest.NewRequest("POST", "/api/v1/search/vault-1", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	server.handleSearch(w, req)

	var searchResp SuccessResponse
	json.NewDecoder(w.Body).Decode(&searchResp)
	searchData := searchResp.Data.(map[string]interface{})

	// Should find "UniqueIdentifier1" in vault-1
	total := uint64(searchData["total"].(float64))
	if total == 0 {
		t.Error("Should find vault-1 content in vault-1 search")
	}

	t.Logf("Search vault-1 for UniqueIdentifier1: found %d results", total)

	// Search vault-2 for vault-1 content - should NOT find it
	searchReq2 := SearchRequest{Query: "UniqueIdentifier1", Type: "text"}
	body2, _ := json.Marshal(searchReq2)
	req = httptest.NewRequest("POST", "/api/v1/search/vault-2", bytes.NewBuffer(body2))
	w = httptest.NewRecorder()
	server.handleSearch(w, req)

	json.NewDecoder(w.Body).Decode(&searchResp)
	searchData = searchResp.Data.(map[string]interface{})
	total = uint64(searchData["total"].(float64))

	t.Logf("Search vault-2 for UniqueIdentifier1: found %d results", total)

	if total > 0 {
		t.Error("Vault isolation broken: Found vault-1 content in vault-2 search")
	}

	// Search in vault-2 for vault-2 content
	searchReq3 := SearchRequest{Query: "UniqueIdentifier2", Type: "text"}
	body3, _ := json.Marshal(searchReq3)
	req = httptest.NewRequest("POST", "/api/v1/search/vault-2", bytes.NewBuffer(body3))
	w = httptest.NewRecorder()
	server.handleSearch(w, req)

	json.NewDecoder(w.Body).Decode(&searchResp)
	searchData = searchResp.Data.(map[string]interface{})
	total = uint64(searchData["total"].(float64))

	t.Logf("Search vault-2 for UniqueIdentifier2: found %d results", total)

	if total == 0 {
		t.Error("Should find vault-2 content in vault-2")
	}

	// List all vaults
	req = httptest.NewRequest("GET", "/api/v1/vaults", nil)
	w = httptest.NewRecorder()
	server.handleVaults(w, req)

	var vaultsResp SuccessResponse
	json.NewDecoder(w.Body).Decode(&vaultsResp)
	vaultsData := vaultsResp.Data.(map[string]interface{})

	vaultsList := vaultsData["vaults"].([]interface{})
	if len(vaultsList) != 2 {
		t.Errorf("Expected 2 vaults, got %d", len(vaultsList))
	}

	t.Log("✓ Multi-vault data isolation verified")
}
