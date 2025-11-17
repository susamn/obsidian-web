package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/vault"
)

func TestParseVaultPath(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name        string
		urlPath     string
		prefix      string
		expectOK    bool
		expectVault string
		expectPath  string
	}{
		{
			name:        "Valid path",
			urlPath:     "/api/v1/files/my-vault/note.md",
			prefix:      "/api/v1/files/",
			expectOK:    true,
			expectVault: "my-vault",
			expectPath:  "note.md",
		},
		{
			name:        "Nested path",
			urlPath:     "/api/v1/files/my-vault/folder/subfolder/note.md",
			prefix:      "/api/v1/files/",
			expectOK:    true,
			expectVault: "my-vault",
			expectPath:  "folder/subfolder/note.md",
		},
		{
			name:     "Missing vault",
			urlPath:  "/api/v1/files/",
			prefix:   "/api/v1/files/",
			expectOK: false,
		},
		{
			name:     "Directory traversal attempt",
			urlPath:  "/api/v1/files/my-vault/../etc/passwd",
			prefix:   "/api/v1/files/",
			expectOK: false,
		},
		{
			name:     "Only vault, no file",
			urlPath:  "/api/v1/files/my-vault",
			prefix:   "/api/v1/files/",
			expectOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vaultID, filePath, ok := server.parseVaultPath(tt.urlPath, tt.prefix)

			if ok != tt.expectOK {
				t.Errorf("Expected ok=%v, got %v", tt.expectOK, ok)
			}

			if ok {
				if vaultID != tt.expectVault {
					t.Errorf("Expected vault '%s', got '%s'", tt.expectVault, vaultID)
				}

				if filePath != tt.expectPath {
					t.Errorf("Expected path '%s', got '%s'", tt.expectPath, filePath)
				}
			}
		})
	}
}

func TestHandleGetFile(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.md")
	testContent := "# Test Note\n\nThis is a test."
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
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

	// Start vault
	if err := v.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}
	defer v.Stop()

	// Wait for vault to be ready
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

	// Test successful file retrieval
	req := httptest.NewRequest("GET", "/api/v1/files/test-vault/test.md", nil)
	w := httptest.NewRecorder()

	server.handleGetFile(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Test non-existent file
	req = httptest.NewRequest("GET", "/api/v1/files/test-vault/nonexistent.md", nil)
	w = httptest.NewRecorder()

	server.handleGetFile(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	// Test non-existent vault
	req = httptest.NewRequest("GET", "/api/v1/files/nonexistent-vault/test.md", nil)
	w = httptest.NewRecorder()

	server.handleGetFile(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	// Test method not allowed
	req = httptest.NewRequest("POST", "/api/v1/files/test-vault/test.md", nil)
	w = httptest.NewRecorder()

	server.handleGetFile(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetRaw(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("raw content"), 0644); err != nil {
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

	// Test successful raw file retrieval
	req := httptest.NewRequest("GET", "/api/v1/raw/test-vault/test.txt", nil)
	w := httptest.NewRecorder()

	server.handleGetRaw(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test method not allowed
	req = httptest.NewRequest("POST", "/api/v1/raw/test-vault/test.txt", nil)
	w = httptest.NewRecorder()

	server.handleGetRaw(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestExtractVaultID(t *testing.T) {
	server := &Server{}

	tests := []struct {
		name     string
		urlPath  string
		prefix   string
		expected string
	}{
		{
			name:     "Simple vault ID",
			urlPath:  "/api/v1/search/my-vault",
			prefix:   "/api/v1/search/",
			expected: "my-vault",
		},
		{
			name:     "With trailing slash",
			urlPath:  "/api/v1/search/my-vault/",
			prefix:   "/api/v1/search/",
			expected: "my-vault",
		},
		{
			name:     "Empty path",
			urlPath:  "/api/v1/search/",
			prefix:   "/api/v1/search/",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.extractVaultID(tt.urlPath, tt.prefix)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
