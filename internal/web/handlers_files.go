package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FileResponse represents a file's content
type FileResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

// handleGetFile godoc
// @Summary Get a file from a vault
// @Description Get the content of a file from a vault
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param path path string true "File path"
// @Success 200 {object} FileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/{vault}/{path} [get]
func (s *Server) handleGetFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse vault and path from URL
	vaultID, filePath, ok := s.parseVaultPath(r.URL.Path, "/api/v1/files/")
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid path format")
		return
	}

	// Get vault
	v, ok := s.getVault(vaultID)
	if !ok {
		writeError(w, http.StatusNotFound, "Vault not found")
		return
	}

	// Check vault is active
	if !v.IsActive() {
		writeError(w, http.StatusServiceUnavailable, "Vault not active")
		return
	}

	// Read file
	content, size, err := s.readVaultFile(v, filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return
	}

	// Return file content
	writeSuccess(w, FileResponse{
		Path:    filePath,
		Content: content,
		Size:    size,
	})
}

// handleGetRaw godoc
// @Summary Get a raw file from a vault
// @Description Get the raw content of a file from a vault
// @Tags files
// @Produce octet-stream
// @Param vault path string true "Vault ID"
// @Param path path string true "File path"
// @Success 200 {string} string "Raw file content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/raw/{vault}/{path} [get]
func (s *Server) handleGetRaw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse vault and path
	vaultID, filePath, ok := s.parseVaultPath(r.URL.Path, "/api/v1/raw/")
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid path format")
		return
	}

	// Get vault
	v, ok := s.getVault(vaultID)
	if !ok {
		writeError(w, http.StatusNotFound, "Vault not found")
		return
	}

	// Check vault is active
	if !v.IsActive() {
		writeError(w, http.StatusServiceUnavailable, "Vault not active")
		return
	}

	// Build full file path
	fullPath := s.buildVaultFilePath(v, filePath)

	// Serve raw file
	http.ServeFile(w, r, fullPath)
}

// parseVaultPath extracts vault ID and file path from URL
func (s *Server) parseVaultPath(urlPath, prefix string) (vaultID, filePath string, ok bool) {
	// Remove prefix
	path := strings.TrimPrefix(urlPath, prefix)
	if path == "" {
		return "", "", false
	}

	// Split into vault and file path
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return "", "", false
	}

	vaultID = parts[0]
	filePath = parts[1]

	// Security: prevent directory traversal
	if strings.Contains(filePath, "..") {
		return "", "", false
	}

	return vaultID, filePath, true
}

// buildVaultFilePath constructs the full filesystem path for a file
func (s *Server) buildVaultFilePath(v interface{ VaultID() string }, filePath string) string {
	// Get vault path from config
	s.mu.RLock()
	defer s.mu.RUnlock()

	vaultID := v.VaultID()
	for _, vaultCfg := range s.config.Vaults {
		if vaultCfg.ID == vaultID {
			localCfg := vaultCfg.Storage.GetLocalConfig()
			if localCfg != nil {
				return filepath.Join(localCfg.Path, filePath)
			}
		}
	}

	return ""
}

// readVaultFile reads a file from vault and returns content and size
func (s *Server) readVaultFile(v interface{ VaultID() string }, filePath string) (content string, size int64, err error) {
	fullPath := s.buildVaultFilePath(v, filePath)
	if fullPath == "" {
		return "", 0, fmt.Errorf("vault path not found")
	}

	// Check file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", 0, err
	}

	// Read file content
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", 0, err
	}

	return string(data), info.Size(), nil
}
