package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/sse"
	"github.com/susamn/obsidian-web/internal/vault"
)

func TestHandleSSE(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup
	tempDir := t.TempDir()
	indexDir := t.TempDir()
	vaultCfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		DBPath:    tempDir,
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
	server.Start() // Starts SSE manager
	defer server.Stop()

	// Test 1: Successful connection
	req := httptest.NewRequest("GET", "/api/v1/sse/test-vault", nil)
	w := httptest.NewRecorder()

	// Use a cancelable context for the request to simulate client disconnect
	reqCtx, reqCancel := context.WithCancel(ctx)
	req = req.WithContext(reqCtx)

	go func() {
		time.Sleep(100 * time.Millisecond)
		// Broadcast an event to verify stream
		server.sseManager.BroadcastFileEvent("test-vault", "test.md", sse.EventFileCreated)
		time.Sleep(100 * time.Millisecond)
		reqCancel() // Stop the handler
	}()

	server.handleSSE(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "event: connected") {
		t.Error("Expected connected event")
	}
	if !strings.Contains(body, "event: file_created") {
		t.Error("Expected file_created event")
	}

	// Test 2: Vault not found
	req = httptest.NewRequest("GET", "/api/v1/sse/non-existent", nil)
	w = httptest.NewRecorder()
	server.handleSSE(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	// Test 3: Method not allowed
	req = httptest.NewRequest("POST", "/api/v1/sse/test-vault", nil)
	w = httptest.NewRecorder()
	server.handleSSE(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleSSEStats(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		DBPath:    tempDir,
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, _ := vault.NewVault(ctx, vaultCfg)
	vaults := map[string]*vault.Vault{
		"test-vault": v,
	}
	cfg := &config.Config{Server: config.ServerConfig{Host: "localhost", Port: 8080}}

	server := NewServer(ctx, cfg, vaults)
	server.Start()
	defer server.Stop()

	req := httptest.NewRequest("GET", "/api/v1/sse/stats", nil)
	w := httptest.NewRecorder()

	server.handleSSEStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response missing 'data' field or invalid type")
	}

	if _, ok := data["total_clients"]; !ok {
		t.Error("Expected total_clients in stats")
	}
}
