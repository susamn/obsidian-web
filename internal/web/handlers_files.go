package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/susamn/obsidian-web/internal/db"
	"github.com/susamn/obsidian-web/internal/logger"
)

// FileResponse represents a file's content
type FileResponse struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

// COMMENTED OUT - UNUSED: Path-based file access (replaced by ID-based handleGetFileByID)
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
/*
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
*/

// COMMENTED OUT - UNUSED: Raw file serving (not used by frontend)
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
/*
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
*/

// handleGetFileByID godoc
// @Summary Get a file from a vault by node ID
// @Description Get the content of a file from a vault using its node ID. Returns file content along with metadata including relative path (read-only).
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param id path string true "Node ID"
// @Success 200 {object} object{content=string,path=string,id=string,name=string}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/by-id/{vault}/{id} [get]
func (s *Server) handleGetFileByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse vault and ID from URL
	vaultID, nodeID, ok := s.parseVaultPath(r.URL.Path, "/api/v1/files/by-id/")
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid path format")
		return
	}

	// Validate vault and get DB service
	v, dbService, ok := s.validateAndGetVaultWithDB(w, vaultID)
	if !ok {
		return
	}

	// Get file entry by ID (contains both path and metadata)
	fileEntry, ok := s.getFileEntryByID(w, dbService, vaultID, nodeID)
	if !ok {
		return
	}

	// Read file content
	content, _, ok := s.readFileContentBinary(w, v, fileEntry.Path)
	if !ok {
		return
	}

	// Return file content with metadata (path is read-only, never used as input)
	writeSuccess(w, map[string]interface{}{
		"content": string(content),
		"path":    fileEntry.Path, // Relative path for UI navigation only
		"id":      fileEntry.ID,
		"name":    fileEntry.Name,
	})
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

// readVaultFile reads a file from vault and returns content and size
func (s *Server) readVaultFileInBinary(v interface{ VaultID() string }, filePath string) ([]byte, int64, error) {
	fullPath := s.buildVaultFilePath(v, filePath)
	if fullPath == "" {
		return nil, 0, fmt.Errorf("vault path not found")
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, 0, err
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, 0, err
	}

	return data, info.Size(), nil
}

// handleGetTree godoc
// @Summary Get directory tree
// @Description Get the full recursive directory tree for a vault. Returns all files and folders with their complete hierarchy.
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param path query string false "Directory path (deprecated - always returns full tree from root)"
// @Success 200 {object} object "Array of tree nodes with full recursive children"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/tree/{vault} [get]
func (s *Server) handleGetTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID from path
	vaultID := s.extractVaultIDFromPath(r.URL.Path, "/api/v1/files/tree/")
	if vaultID == "" {
		writeError(w, http.StatusBadRequest, "Vault ID required")
		return
	}

	// Validate vault and get explorer service
	_, explorerSvc, ok := s.validateAndGetVaultWithExplorer(w, vaultID)
	if !ok {
		return
	}

	// Get full recursive tree (ignoring path parameter for now)
	// This returns the complete tree structure with all files and folders
	nodes, err := explorerSvc.GetFullTree()
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to get tree: %v", err))
		return
	}

	writeSuccess(w, map[string]interface{}{
		"vault_id": vaultID,
		"nodes":    nodes,
		"count":    len(nodes),
	})
}

// COMMENTED OUT - UNUSED: Lazy-loaded children (frontend now uses full tree)
// handleGetChildren godoc
// @Summary Get directory children
// @Description Get the direct children of a directory (lazy-loaded)
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param path query string false "Directory path (empty for root)"
// @Success 200 {object} object "Array of child nodes"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/children/{vault} [get]
/*
func (s *Server) handleGetChildren(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID from path
	vaultID := s.extractVaultIDFromPath(r.URL.Path, "/api/v1/files/children/")
	if vaultID == "" {
		writeError(w, http.StatusBadRequest, "Vault ID required")
		return
	}

	// Validate vault and get explorer service
	_, explorerSvc, ok := s.validateAndGetVaultWithExplorer(w, vaultID)
	if !ok {
		return
	}

	// Get path from query parameter
	path := r.URL.Query().Get("path")

	// Get children
	children, err := explorerSvc.GetChildren(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to get children: %v", err))
		return
	}

	writeSuccess(w, map[string]interface{}{
		"path":     path,
		"children": children,
		"count":    len(children),
	})
}
*/

// handleGetMetadata godoc
// @Summary Get file/directory metadata
// @Description Get metadata for a file or directory
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param path query string true "File or directory path"
// @Success 200 {object} object "Metadata"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/meta/{vault} [get]
func (s *Server) handleGetMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID from path
	vaultID := s.extractVaultIDFromPath(r.URL.Path, "/api/v1/files/meta/")
	if vaultID == "" {
		writeError(w, http.StatusBadRequest, "Vault ID required")
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

	// Get explorer service
	explorerSvc := v.GetExplorerService()
	if explorerSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "Explorer service not available")
		return
	}

	// Get path from query parameter
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "Path parameter required")
		return
	}

	// Get metadata
	metadata, err := explorerSvc.GetMetadata(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to get metadata: %v", err))
		return
	}

	writeSuccess(w, metadata)
}

// COMMENTED OUT - UNUSED: Manual tree refresh (removed from frontend)
// handleRefreshTree godoc
// @Summary Refresh directory tree
// @Description Manually refresh the cached directory tree for a path
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param path query string false "Directory path (empty for root)"
// @Success 200 {object} object "Success message"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/refresh/{vault} [post]
/*
func (s *Server) handleRefreshTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID from path
	vaultID := s.extractVaultIDFromPath(r.URL.Path, "/api/v1/files/refresh/")
	if vaultID == "" {
		writeError(w, http.StatusBadRequest, "Vault ID required")
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

	// Get explorer service
	explorerSvc := v.GetExplorerService()
	if explorerSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "Explorer service not available")
		return
	}

	// Get path from query parameter
	path := r.URL.Query().Get("path")

	// Refresh path
	if err := explorerSvc.RefreshPath(path); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to refresh: %v", err))
		return
	}

	writeSuccess(w, map[string]interface{}{
		"message": "Tree refreshed successfully",
		"path":    path,
	})
}
*/

// COMMENTED OUT - UNUSED: Lazy-loaded children by ID (frontend now uses full tree)
// handleGetChildrenByID godoc
// @Summary Get directory children by ID
// @Description Get the direct children of a directory by node ID
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param id path string true "Node ID (parent directory ID)"
// @Success 200 {object} object "Array of child nodes"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/children-by-id/{vault}/{id} [get]
/*
func (s *Server) handleGetChildrenByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID and node ID from path
	vaultID, nodeID, ok := s.parseVaultPath(r.URL.Path, "/api/v1/files/children-by-id/")
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

	// Get DB service
	dbService := v.GetDBService()
	if dbService == nil {
		writeError(w, http.StatusServiceUnavailable, "Database service not available")
		return
	}

	// Get children by parent ID from database
	children, err := dbService.GetFileEntriesByParentID(&nodeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to get children: %v", err))
		return
	}

	// Convert to explorer nodes for consistency
	var nodes []map[string]interface{}
	for _, entry := range children {
		nodes = append(nodes, map[string]interface{}{
			"metadata": map[string]interface{}{
				"id":           entry.ID,
				"name":         entry.Name,
				"is_directory": entry.IsDir,
				"is_markdown":  strings.HasSuffix(entry.Name, ".md"),
				"type":         map[bool]string{true: "directory", false: "file"}[entry.IsDir],
			},
		})
	}

	writeSuccess(w, map[string]interface{}{
		"id":       nodeID,
		"children": nodes,
		"count":    len(nodes),
	})
}
*/

// COMMENTED OUT - UNUSED: Lazy-loaded tree by ID (frontend now uses full tree)
// handleGetTreeByID godoc
// @Summary Get directory tree by ID
// @Description Get the directory tree (lazy-loaded) for a node ID
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Param id path string true "Node ID (directory ID)"
// @Success 200 {object} object "Tree node with metadata"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/tree-by-id/{vault}/{id} [get]
/*
func (s *Server) handleGetTreeByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID and node ID from path
	vaultID, nodeID, ok := s.parseVaultPath(r.URL.Path, "/api/v1/files/tree-by-id/")
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

	// Get DB service
	dbService := v.GetDBService()
	if dbService == nil {
		writeError(w, http.StatusServiceUnavailable, "Database service not available")
		return
	}

	// Get node entry from database
	nodeEntry, err := dbService.GetFileEntryByID(nodeID)
	if err != nil || nodeEntry == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("Node not found: %v", err))
		return
	}

	// Get children
	children, err := dbService.GetFileEntriesByParentID(&nodeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to get children: %v", err))
		return
	}

	// Build node response
	var childNodes []map[string]interface{}
	for _, entry := range children {
		childNodes = append(childNodes, map[string]interface{}{
			"metadata": map[string]interface{}{
				"id":           entry.ID,
				"name":         entry.Name,
				"is_directory": entry.IsDir,
				"is_markdown":  strings.HasSuffix(entry.Name, ".md"),
				"type":         map[bool]string{true: "directory", false: "file"}[entry.IsDir],
			},
		})
	}

	writeSuccess(w, map[string]interface{}{
		"metadata": map[string]interface{}{
			"id":           nodeEntry.ID,
			"name":         nodeEntry.Name,
			"is_directory": nodeEntry.IsDir,
			"is_markdown":  strings.HasSuffix(nodeEntry.Name, ".md"),
			"type":         map[bool]string{true: "directory", false: "file"}[nodeEntry.IsDir],
		},
		"children": childNodes,
		"loaded":   true,
	})
}
*/

// handleForceReindex godoc
// @Summary Trigger vault reindex
// @Description Trigger a full vault reindex using the reconciliation service. This will disable all files, clear caches and index, then rebuild everything from the filesystem.
// @Tags files
// @Produce json
// @Param vault path string true "Vault ID"
// @Success 200 {object} object "Reindex started message"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/files/reindex/{vault} [post]
func (s *Server) handleForceReindex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID from path
	vaultID := s.extractVaultIDFromPath(r.URL.Path, "/api/v1/files/reindex/")
	if vaultID == "" {
		writeError(w, http.StatusBadRequest, "Vault ID required")
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

	// Trigger reindex via reconciliation service (runs asynchronously)
	v.TriggerReindex()

	writeSuccess(w, map[string]interface{}{
		"message":  "Reindex started",
		"vault_id": vaultID,
	})
}

// handleGetAsset godoc
// @Summary Get asset file by ID
// @Description Serves a raw asset file (image, pdf, etc.) by file ID
// @Tags assets
// @Produce octet-stream
// @Param vault path string true "Vault ID"
// @Param id path string true "File ID"
// @Success 200 {string} string "Raw file content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/assets/{vault}/{id} [get]
func (s *Server) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse vault and file ID from URL
	vaultID, fileID, ok := s.parseVaultPath(r.URL.Path, "/api/v1/assets/")
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid path format")
		return
	}

	// Validate vault and get DB service
	v, dbService, ok := s.validateAndGetVaultWithDB(w, vaultID)
	if !ok {
		return
	}

	// Get file entry by ID
	fileEntry, ok := s.getFileEntryByID(w, dbService, vaultID, fileID)
	if !ok {
		return
	}

	if fileEntry.IsDir {
		writeError(w, http.StatusNotFound, "Asset not found or is a directory")
		return
	}

	// Get file type information for proper MIME type
	var mimeType string
	if fileEntry.FileTypeID != nil {
		// Query the file_types table for MIME type
		fileType, err := dbService.GetFileTypeByID(*fileEntry.FileTypeID)
		if err == nil && fileType != nil {
			// Map file type to MIME type
			mimeType = getMimeType(*fileType)
		}
	}

	// Build full file path
	fullPath := s.buildVaultFilePath(v, fileEntry.Path)
	if fullPath == "" {
		writeError(w, http.StatusNotFound, "Asset path not found")
		return
	}

	// Set Content-Type header if we have a MIME type
	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}

	// Set Cache-Control header for better performance
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Serve the file
	http.ServeFile(w, r, fullPath)
}

// getMimeType returns the MIME type for a given FileType
func getMimeType(ft db.FileType) string {
	switch ft {
	case db.FileTypeMarkdown:
		return "text/markdown"
	case db.FileTypePNG:
		return "image/png"
	case db.FileTypeJPEG, db.FileTypeJPG:
		return "image/jpeg"
	case db.FileTypeGIF:
		return "image/gif"
	case db.FileTypeWebP:
		return "image/webp"
	case db.FileTypeSVG:
		return "image/svg+xml"
	case db.FileTypePDF:
		return "application/pdf"
	case db.FileTypeTXT:
		return "text/plain"
	case db.FileTypeJSON:
		return "application/json"
	case db.FileTypeYAML:
		return "application/x-yaml"
	case db.FileTypeXML:
		return "application/xml"
	case db.FileTypeCSV:
		return "text/csv"
	default:
		return "application/octet-stream"
	}
}

// extractVaultIDFromPath extracts vault ID from URL path with a given prefix
func (s *Server) extractVaultIDFromPath(urlPath, prefix string) string {
	path := strings.TrimPrefix(urlPath, prefix)
	path = strings.TrimSuffix(path, "/")
	return path
}

// CreateFileRequest represents a request to create a file or folder
type CreateFileRequest struct {
	VaultID  string `json:"vault_id"`
	ParentID string `json:"parent_id,omitempty"` // Optional parent folder ID
	Name     string `json:"name"`                // File or folder name
	IsFolder bool   `json:"is_folder"`           // true for folder, false for file
	Content  string `json:"content,omitempty"`   // File content (only for files)
}

// handleCreateFile godoc
// @Summary Create a new file or folder
// @Description Create a new file or folder in a vault
// @Tags files
// @Accept json
// @Produce json
// @Param request body CreateFileRequest true "Create file request"
// @Success 200 {object} object{message=string,id=string,path=string}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/file/create [post]
func (s *Server) handleCreateFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse request body
	var req CreateFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Validate required fields
	if req.VaultID == "" {
		writeError(w, http.StatusBadRequest, "vault_id is required")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Validate vault and get services
	v, dbService, ok := s.validateAndGetVaultWithDB(w, req.VaultID)
	if !ok {
		return
	}

	// Build the file path
	var targetPath string
	if req.ParentID != "" {
		// Get parent folder entry to build path
		parentEntry, err := dbService.GetFileEntryByID(req.ParentID)
		if err != nil || parentEntry == nil {
			writeError(w, http.StatusNotFound, "Parent folder not found")
			return
		}
		if !parentEntry.IsDir {
			writeError(w, http.StatusBadRequest, "Parent must be a directory")
			return
		}
		targetPath = filepath.Join(parentEntry.Path, req.Name)
	} else {
		// Create at root
		targetPath = req.Name
	}

	// Security: prevent directory traversal
	if strings.Contains(targetPath, "..") {
		writeError(w, http.StatusBadRequest, "Invalid path: directory traversal not allowed")
		return
	}

	// Auto-add .md extension for files if not present
	if !req.IsFolder && !strings.HasSuffix(strings.ToLower(req.Name), ".md") {
		targetPath += ".md"
		req.Name += ".md"
	}

	// Build full filesystem path
	fullPath := s.buildVaultFilePath(v, targetPath)
	if fullPath == "" {
		writeError(w, http.StatusInternalServerError, "Failed to build file path")
		return
	}

	// Check if file/folder already exists
	if _, err := os.Stat(fullPath); err == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("%s already exists", map[bool]string{true: "Folder", false: "File"}[req.IsFolder]))
		return
	}

	// Create the file or folder
	if req.IsFolder {
		// Create directory
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create folder: %v", err))
			return
		}
	} else {
		// Ensure parent directory exists
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create parent directory: %v", err))
			return
		}

		// Create file with content
		content := req.Content
		if content == "" {
			content = "# " + strings.TrimSuffix(req.Name, ".md") + "\n\n"
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create file: %v", err))
			return
		}
	}

	// The file will be picked up by the file watcher and indexed automatically
	// Wait a moment for the watcher to process it
	time.Sleep(100 * time.Millisecond)

	// Try to get the new file entry from database
	var fileID string
	// Try a few times to get the file entry (watcher might take a moment)
	for i := 0; i < 10; i++ {
		entry, err := dbService.GetFileEntryByPath(targetPath)
		if err == nil && entry != nil {
			fileID = entry.ID
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	logger.WithFields(map[string]interface{}{
		"vault_id":  req.VaultID,
		"path":      targetPath,
		"is_folder": req.IsFolder,
		"file_id":   fileID,
	}).Info("Created file/folder")

	writeSuccess(w, map[string]interface{}{
		"message":   fmt.Sprintf("%s created successfully", map[bool]string{true: "Folder", false: "File"}[req.IsFolder]),
		"id":        fileID,
		"path":      targetPath,
		"name":      req.Name,
		"is_folder": req.IsFolder,
	})
}
