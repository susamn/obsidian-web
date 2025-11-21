package web

import (
	"fmt"
	"net/http"

	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/render"
)

// RenderedFileResponse represents the server-side rendered file content
type RenderedFileResponse struct {
	Path  string `json:"path"`
	HTML  string `json:"html"`
	Error string `json:"error,omitempty"`
}

// handleSSRFileByID godoc
// @Summary Get a server-side rendered file from a vault by node ID
// @Description Get the content of a markdown file rendered to HTML server-side using Goldmark
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param id path string true "Node ID"
// @Success 200 {object} RenderedFileResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/ssr/by-id/{vault}/{id} [get]
func (s *Server) handleSSRFileByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse vault and ID from URL
	vaultID, nodeID, ok := s.parseVaultPath(r.URL.Path, "/api/v1/files/ssr/by-id/")
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

	// Get the DBService to find the file path by ID
	dbService := v.GetDBService()
	if dbService == nil {
		writeError(w, http.StatusInternalServerError, "Database service not available")
		return
	}

	// Find file path by ID
	filePath, err := dbService.GetFilePathByID(nodeID)
	if err != nil {
		logger.WithError(err).WithFields(map[string]interface{}{
			"vault_id": vaultID,
			"node_id":  nodeID,
		}).Warn("Failed to find file path by ID for SSR")
		writeError(w, http.StatusNotFound, "File not found")
		return
	}

	// Read file content (as binary to preserve encoding)
	contentBytes, _, err := s.readVaultFileInBinary(v, filePath)
	if err != nil {
		logger.WithError(err).WithFields(map[string]interface{}{
			"vault_id":  vaultID,
			"file_path": filePath,
		}).Warn("Failed to read file for SSR")
		writeError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return
	}

	// Convert bytes to string for rendering
	content := string(contentBytes)

	// Create a new renderer instance
	renderer := render.NewRenderer()

	// Render markdown to HTML
	html, err := renderer.RenderMarkdownToString(content)
	if err != nil {
		logger.WithError(err).WithFields(map[string]interface{}{
			"vault_id":  vaultID,
			"file_path": filePath,
		}).Error("Failed to render markdown for SSR")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to render markdown: %v", err))
		return
	}

	// Return rendered content
	writeSuccess(w, RenderedFileResponse{
		Path: filePath,
		HTML: html,
	})
}
