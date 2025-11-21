package web

import (
	"fmt"
	"net/http"
	"os"

	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/render"
)

// handleStructuredRenderByID godoc
// @Summary Get a structured rendered file from a vault by node ID
// @Description Get the content of a markdown file with structured metadata (headings, tags, wikilinks, etc.)
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param id path string true "Node ID"
// @Success 200 {object} render.FileContentResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/sr/by-id/{vault}/{id} [get]
func (s *Server) handleStructuredRenderByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse vault and ID from URL
	vaultID, nodeID, ok := s.parseVaultPath(r.URL.Path, "/api/v1/files/sr/by-id/")
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
		}).Warn("Failed to find file path by ID for structured rendering")
		writeError(w, http.StatusNotFound, "File not found")
		return
	}

	// Read file content
	contentBytes, _, err := s.readVaultFileInBinary(v, filePath)
	if err != nil {
		logger.WithError(err).WithFields(map[string]interface{}{
			"vault_id":  vaultID,
			"file_path": filePath,
		}).Warn("Failed to read file for structured rendering")
		writeError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return
	}

	// Get file metadata for timestamps
	fullPath := s.buildVaultFilePath(v, filePath)
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		logger.WithError(err).WithFields(map[string]interface{}{
			"vault_id":  vaultID,
			"file_path": filePath,
		}).Warn("Failed to get file info for structured rendering")
		writeError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return
	}

	// Convert bytes to string
	content := string(contentBytes)

	// Create file resolver (pass nil for now as we don't have full resolver implementation)
	var resolver render.FileResolver = nil

	// Create structured renderer
	renderer := render.NewStructuredRenderer(resolver)

	// Process markdown
	response, err := renderer.ProcessMarkdown(
		content,
		vaultID,
		nodeID,
		fileInfo.ModTime(), // Use ModTime for both created and modified for now
		fileInfo.ModTime(),
	)
	if err != nil {
		logger.WithError(err).WithFields(map[string]interface{}{
			"vault_id":  vaultID,
			"file_path": filePath,
		}).Error("Failed to process markdown for structured rendering")
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to process markdown: %v", err))
		return
	}

	// Return structured content
	writeSuccess(w, response)
}

// Note: DBFileResolver is commented out for now until we implement
// the required methods in the database service
// TODO: Implement GetFileIDByName, GetBacklinksByFileID, and GetTagCount in DBService
