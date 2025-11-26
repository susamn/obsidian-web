package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/vault"
)

func TestHandleHealth(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	server := NewServer(ctx, cfg, make(map[string]*vault.Vault))

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Test method not allowed
	req = httptest.NewRequest("POST", "/api/v1/health", nil)
	w = httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// COMMENTED OUT - handleMetrics is currently disabled
/*
func TestHandleMetrics_VaultSpecific(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

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

	v, _ := vault.NewVault(ctx, vaultCfg)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	vaults := map[string]*vault.Vault{
		"test-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Test vault-specific metrics
	req := httptest.NewRequest("GET", "/api/v1/metrics/test-vault", nil)
	w := httptest.NewRecorder()

	server.handleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Test non-existent vault
	req = httptest.NewRequest("GET", "/api/v1/metrics/nonexistent", nil)
	w = httptest.NewRecorder()

	server.handleMetrics(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}
*/

// COMMENTED OUT - handleMetrics is currently disabled
/*
func TestHandleSystemMetrics(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

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

	v, _ := vault.NewVault(ctx, vaultCfg)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	vaults := map[string]*vault.Vault{
		"test-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Test system metrics (no vault ID)
	req := httptest.NewRequest("GET", "/api/v1/metrics/", nil)
	w := httptest.NewRecorder()

	server.handleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response has system and vaults fields
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Response data is not a map")
	}

	if _, ok := data["system"]; !ok {
		t.Error("Expected 'system' field in response")
	}

	if _, ok := data["vaults"]; !ok {
		t.Error("Expected 'vaults' field in response")
	}

	if _, ok := data["uptime"]; !ok {
		t.Error("Expected 'uptime' field in response")
	}
}
*/
