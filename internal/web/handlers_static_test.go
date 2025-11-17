package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/vault"
)

func TestHandleIndex(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	server := NewServer(ctx, cfg, make(map[string]*vault.Vault))

	// Test index page
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.handleIndex(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}

	// Test non-root path
	req = httptest.NewRequest("GET", "/other", nil)
	w = httptest.NewRecorder()

	server.handleIndex(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleStatic(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:      "localhost",
			Port:      8080,
			StaticDir: "/tmp",
		},
	}

	server := NewServer(ctx, cfg, make(map[string]*vault.Vault))

	// Test empty path
	req := httptest.NewRequest("GET", "/static/", nil)
	w := httptest.NewRecorder()

	server.handleStatic(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for empty path, got %d", w.Code)
	}

	// Test path with directory traversal
	req = httptest.NewRequest("GET", "/static/../etc/passwd", nil)
	w = httptest.NewRecorder()

	server.handleStatic(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for directory traversal, got %d", w.Code)
	}

	// Test valid file path (file won't exist, but path is valid)
	req = httptest.NewRequest("GET", "/static/test.js", nil)
	w = httptest.NewRecorder()

	server.handleStatic(w, req)

	// Will return 404 since file doesn't exist, but should not return 400
	if w.Code == http.StatusBadRequest {
		t.Error("Should not return bad request for valid path")
	}
}
