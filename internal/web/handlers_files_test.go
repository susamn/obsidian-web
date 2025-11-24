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
			result := server.extractVaultIDFromPath(tt.urlPath, tt.prefix)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestHandleGetFileByID(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()
	dbDir := t.TempDir()

	// Create a test file in a nested directory
	nestedDir := filepath.Join(tempDir, "folder", "subfolder")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	testFile := filepath.Join(nestedDir, "test.md")
	testContent := "# Test Note\n\nThis is a nested test file."
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	vaultCfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		DBPath:    dbDir,
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

	// Force reindex to populate database
	if err := v.ForceReindex(); err != nil {
		t.Fatalf("Failed to reindex vault: %v", err)
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

	// Get the DB service to find a file entry
	dbService := v.GetDBService()
	if dbService == nil {
		t.Fatal("DB service not available")
	}

	// Fetch the file entry by path to get its ID
	fileEntry, err := dbService.GetFileEntryByPath("folder/subfolder/test.md")
	if err != nil || fileEntry == nil {
		t.Fatalf("Failed to find test file in database: %v", err)
	}

	// Test successful file retrieval by ID
	t.Run("Success - get file by ID with metadata", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/files/by-id/test-vault/"+fileEntry.ID, nil)
		w := httptest.NewRecorder()

		server.handleGetFileByID(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
			return
		}

		// Parse JSON response
		body := w.Body.String()

		// Check that response contains expected fields
		if !contains(body, "\"content\":") {
			t.Error("Response missing 'content' field")
		}
		if !contains(body, "\"path\":") {
			t.Error("Response missing 'path' field")
		}
		if !contains(body, "\"id\":") {
			t.Error("Response missing 'id' field")
		}
		if !contains(body, "\"name\":") {
			t.Error("Response missing 'name' field")
		}

		// Check that path is the relative path (read-only)
		if !contains(body, "folder/subfolder/test.md") {
			t.Error("Response does not contain correct relative path")
		}

		// Check that content is included
		if !contains(body, "Test Note") {
			t.Error("Response does not contain file content")
		}
	})

	// Test non-existent file ID
	t.Run("Failure - non-existent file ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/files/by-id/test-vault/non-existent-id", nil)
		w := httptest.NewRecorder()

		server.handleGetFileByID(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	// Test non-existent vault
	t.Run("Failure - non-existent vault", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/files/by-id/nonexistent-vault/"+fileEntry.ID, nil)
		w := httptest.NewRecorder()

		server.handleGetFileByID(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	// Test method not allowed
	t.Run("Failure - method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/files/by-id/test-vault/"+fileEntry.ID, nil)
		w := httptest.NewRecorder()

		server.handleGetFileByID(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})

	// Test invalid path format
	t.Run("Failure - invalid path format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/files/by-id/", nil)
		w := httptest.NewRecorder()

		server.handleGetFileByID(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// createJSONBody creates a JSON request body from a map
func createJSONBody(t *testing.T, data map[string]interface{}) *bytes.Buffer {
	t.Helper()
	body, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}
	return bytes.NewBuffer(body)
}

func TestHandleCreateFile(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()
	dbDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		DBPath:    dbDir,
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

	t.Run("Success - create file at root", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/file/create",
			createJSONBody(t, map[string]interface{}{
				"vault_id":  "test-vault",
				"name":      "new-note",
				"is_folder": false,
				"content":   "# New Note\n\nTest content",
			}))
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
			return
		}

		// Verify file was created with .md extension
		filePath := filepath.Join(tempDir, "new-note.md")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("File was not created")
		}

		// Verify content
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read created file: %v", err)
		}
		if string(content) != "# New Note\n\nTest content" {
			t.Errorf("File content mismatch, got: %s", string(content))
		}

		// Verify response
		body := w.Body.String()
		if !contains(body, "\"name\":\"new-note.md\"") {
			t.Error("Response missing correct name with .md extension")
		}
	})

	t.Run("Success - create file with .md extension already", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/file/create",
			createJSONBody(t, map[string]interface{}{
				"vault_id":  "test-vault",
				"name":      "another-note.md",
				"is_folder": false,
				"content":   "# Another Note",
			}))
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
			return
		}

		// Verify file was created without double .md
		filePath := filepath.Join(tempDir, "another-note.md")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("File was not created")
		}

		// Should not have .md.md
		doubleExtPath := filepath.Join(tempDir, "another-note.md.md")
		if _, err := os.Stat(doubleExtPath); !os.IsNotExist(err) {
			t.Error("File was created with double .md extension")
		}
	})

	t.Run("Success - create folder", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/file/create",
			createJSONBody(t, map[string]interface{}{
				"vault_id":  "test-vault",
				"name":      "new-folder",
				"is_folder": true,
			}))
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
			return
		}

		// Verify folder was created
		folderPath := filepath.Join(tempDir, "new-folder")
		info, err := os.Stat(folderPath)
		if os.IsNotExist(err) {
			t.Error("Folder was not created")
		}
		if err == nil && !info.IsDir() {
			t.Error("Created path is not a directory")
		}
	})

	t.Run("Success - create file with default content", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/file/create",
			createJSONBody(t, map[string]interface{}{
				"vault_id":  "test-vault",
				"name":      "default-content",
				"is_folder": false,
			}))
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
			return
		}

		// Verify file has default content
		filePath := filepath.Join(tempDir, "default-content.md")
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read created file: %v", err)
		}
		expectedContent := "# default-content\n\n"
		if string(content) != expectedContent {
			t.Errorf("Expected default content '%s', got '%s'", expectedContent, string(content))
		}
	})

	t.Run("Failure - missing vault_id", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/file/create",
			createJSONBody(t, map[string]interface{}{
				"name":      "test",
				"is_folder": false,
			}))
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("Failure - missing name", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/file/create",
			createJSONBody(t, map[string]interface{}{
				"vault_id":  "test-vault",
				"is_folder": false,
			}))
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("Failure - file already exists", func(t *testing.T) {
		// Create a file first
		existingFile := filepath.Join(tempDir, "existing.md")
		if err := os.WriteFile(existingFile, []byte("existing"), 0644); err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}

		req := httptest.NewRequest("POST", "/api/v1/file/create",
			createJSONBody(t, map[string]interface{}{
				"vault_id":  "test-vault",
				"name":      "existing",
				"is_folder": false,
			}))
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("Expected status 409, got %d", w.Code)
		}
	})

	t.Run("Failure - directory traversal attempt", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/file/create",
			createJSONBody(t, map[string]interface{}{
				"vault_id":  "test-vault",
				"name":      "../../../etc/passwd",
				"is_folder": false,
			}))
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("Failure - method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/file/create", nil)
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})

	t.Run("Failure - invalid vault", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/file/create",
			createJSONBody(t, map[string]interface{}{
				"vault_id":  "nonexistent-vault",
				"name":      "test",
				"is_folder": false,
			}))
		w := httptest.NewRecorder()

		server.handleCreateFile(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}
