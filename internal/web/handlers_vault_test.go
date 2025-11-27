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

func TestHandleVaults(t *testing.T) {
	ctx := context.Background()
	tempDir1 := t.TempDir()
	indexDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	indexDir2 := t.TempDir()

	vaultCfg1 := &config.VaultConfig{
		ID:        "vault-1",
		Name:      "Vault 1",
		Enabled:   true,
		IndexPath: indexDir1 + "/test.bleve",
		DBPath:    tempDir1 + "/test.db",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir1,
			},
		},
	}

	vaultCfg2 := &config.VaultConfig{
		ID:        "vault-2",
		Name:      "Vault 2",
		Enabled:   true,
		IndexPath: indexDir2 + "/test.bleve",
		DBPath:    tempDir2 + "/test.db",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir2,
			},
		},
	}

	v1, _ := vault.NewVault(ctx, vaultCfg1)
	v2, _ := vault.NewVault(ctx, vaultCfg2)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 19876,
		},
	}

	vaults := map[string]*vault.Vault{
		"vault-1": v1,
		"vault-2": v2,
	}

	server := NewServer(ctx, cfg, vaults)

	// Test GET /api/v1/vaults
	req := httptest.NewRequest("GET", "/api/v1/vaults", nil)
	w := httptest.NewRecorder()

	server.handleVaults(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp SuccessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Test method not allowed
	req = httptest.NewRequest("POST", "/api/v1/vaults", nil)
	w = httptest.NewRecorder()

	server.handleVaults(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetVaultInfo(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		DBPath:    tempDir + "/test.db",
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
			Port: 19876,
		},
	}

	vaults := map[string]*vault.Vault{
		"test-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Test GET /api/v1/vaults/:id
	req := httptest.NewRequest("GET", "/api/v1/vaults/test-vault", nil)
	w := httptest.NewRecorder()

	server.handleVaultOps(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test non-existent vault
	req = httptest.NewRequest("GET", "/api/v1/vaults/nonexistent", nil)
	w = httptest.NewRecorder()

	server.handleVaultOps(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleVaultOperation(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		DBPath:    tempDir + "/test.db",
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
			Port: 19876,
		},
	}

	vaults := map[string]*vault.Vault{
		"test-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Test start operation
	req := httptest.NewRequest("POST", "/api/v1/vaults/test-vault/start", nil)
	w := httptest.NewRecorder()

	server.handleVaultOps(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for start, got %d: %s", w.Code, w.Body.String())
	}

	// Wait a moment
	v.WaitForReady(5 * 1000000000)

	// Test stop operation
	req = httptest.NewRequest("POST", "/api/v1/vaults/test-vault/stop", nil)
	w = httptest.NewRecorder()

	server.handleVaultOps(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for stop, got %d: %s", w.Code, w.Body.String())
	}

	// Test invalid operation
	req = httptest.NewRequest("POST", "/api/v1/vaults/test-vault/invalid", nil)
	w = httptest.NewRecorder()

	server.handleVaultOps(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Test non-existent vault
	req = httptest.NewRequest("POST", "/api/v1/vaults/nonexistent/start", nil)
	w = httptest.NewRecorder()

	server.handleVaultOps(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestStartStopVault(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		DBPath:    tempDir + "/test.db",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, _ := vault.NewVault(ctx, vaultCfg)

	server := &Server{}

	// Start vault
	if err := server.startVault(v); err != nil {
		t.Errorf("Failed to start vault: %v", err)
	}

	// Try to start already active vault
	v.WaitForReady(5 * 1000000000)
	if err := server.startVault(v); err == nil {
		t.Error("Expected error when starting already active vault")
	}

	// Stop vault
	if err := server.stopVault(v); err != nil {
		t.Errorf("Failed to stop vault: %v", err)
	}

	// Stop already stopped vault (should be idempotent)
	if err := server.stopVault(v); err != nil {
		t.Error("Stop should be idempotent")
	}

	// Test resumeVault with stopped vault (basic validation)
	err := server.resumeVault(v)
	// Resume may fail depending on vault state, but shouldn't panic
	_ = err
}
