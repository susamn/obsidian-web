package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/vault"
)

func TestHandleSearch(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tempDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\n\nThis is a test note."), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	vaultCfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
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

	// Start vault and wait for ready
	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}
	defer v.Stop()

	if err := v.WaitForReady(5 * 1000000000); err != nil {
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
		"test-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Test text search
	searchReq := SearchRequest{
		Query: "test",
		Type:  "text",
	}
	body, _ := json.Marshal(searchReq)
	req := httptest.NewRequest("POST", "/api/v1/search/test-vault", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.handleSearch(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Test empty query
	emptyReq := SearchRequest{}
	body, _ = json.Marshal(emptyReq)
	req = httptest.NewRequest("POST", "/api/v1/search/test-vault", bytes.NewBuffer(body))
	w = httptest.NewRecorder()

	server.handleSearch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for empty query, got %d", w.Code)
	}

	// Test method not allowed
	req = httptest.NewRequest("GET", "/api/v1/search/test-vault", nil)
	w = httptest.NewRecorder()

	server.handleSearch(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}

	// Test non-existent vault
	searchReq = SearchRequest{Query: "test"}
	body, _ = json.Marshal(searchReq)
	req = httptest.NewRequest("POST", "/api/v1/search/nonexistent", bytes.NewBuffer(body))
	w = httptest.NewRecorder()

	server.handleSearch(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestConvertSearchResult(t *testing.T) {
	server := &Server{}

	// Test nil result
	resp := server.convertSearchResult(nil)
	if resp.Total != 0 {
		t.Errorf("Expected total 0 for nil result, got %d", resp.Total)
	}
	if len(resp.Results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(resp.Results))
	}
}
