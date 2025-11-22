package web

import (
	"fmt"
	"net/http"

	"github.com/susamn/obsidian-web/internal/db"
	"github.com/susamn/obsidian-web/internal/explorer"
	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/vault"
)

// vaultHelper encapsulates common vault validation logic
type vaultHelper struct {
	vault     *vault.Vault
	dbService *db.DBService
	explorer  *explorer.ExplorerService
}

// validateAndGetVault validates vault exists and is active
// Returns vault and true if valid, writes error and returns false if invalid
func (s *Server) validateAndGetVault(w http.ResponseWriter, vaultID string) (*vault.Vault, bool) {
	// Get vault
	v, ok := s.getVault(vaultID)
	if !ok {
		writeError(w, http.StatusNotFound, "Vault not found")
		return nil, false
	}

	// Check vault is active
	if !v.IsActive() {
		writeError(w, http.StatusServiceUnavailable, "Vault not active")
		return nil, false
	}

	return v, true
}

// validateAndGetVaultWithDB validates vault and gets its DB service
// Returns vault, dbService and true if valid, writes error and returns false if invalid
func (s *Server) validateAndGetVaultWithDB(w http.ResponseWriter, vaultID string) (*vault.Vault, *db.DBService, bool) {
	v, ok := s.validateAndGetVault(w, vaultID)
	if !ok {
		return nil, nil, false
	}

	// Get DB service
	dbService := v.GetDBService()
	if dbService == nil {
		writeError(w, http.StatusInternalServerError, "Database service not available")
		return nil, nil, false
	}

	return v, dbService, true
}

// validateAndGetVaultWithExplorer validates vault and gets its explorer service
// Returns vault, explorer and true if valid, writes error and returns false if invalid
func (s *Server) validateAndGetVaultWithExplorer(w http.ResponseWriter, vaultID string) (*vault.Vault, *explorer.ExplorerService, bool) {
	v, ok := s.validateAndGetVault(w, vaultID)
	if !ok {
		return nil, nil, false
	}

	// Get explorer service
	explorerSvc := v.GetExplorerService()
	if explorerSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "Explorer service not available")
		return nil, nil, false
	}

	return v, explorerSvc, true
}

// getFileEntryByID fetches a file entry from database by ID
// Returns fileEntry and true if found, writes error and returns false if not found
func (s *Server) getFileEntryByID(w http.ResponseWriter, dbService *db.DBService, vaultID, nodeID string) (*db.FileEntry, bool) {
	fileEntry, err := dbService.GetFileEntryByID(nodeID)
	if err != nil {
		logger.WithError(err).WithFields(map[string]interface{}{
			"vault_id": vaultID,
			"node_id":  nodeID,
		}).Warn("Failed to find file by ID")
		writeError(w, http.StatusNotFound, "File not found")
		return nil, false
	}

	if fileEntry == nil {
		writeError(w, http.StatusNotFound, "File not found")
		return nil, false
	}

	return fileEntry, true
}

// getFilePathByID fetches a file path from database by ID
// Returns path and true if found, writes error and returns false if not found
func (s *Server) getFilePathByID(w http.ResponseWriter, dbService *db.DBService, vaultID, nodeID string) (string, bool) {
	filePath, err := dbService.GetFilePathByID(nodeID)
	if err != nil {
		logger.WithError(err).WithFields(map[string]interface{}{
			"vault_id": vaultID,
			"node_id":  nodeID,
		}).Warn("Failed to find file path by ID")
		writeError(w, http.StatusNotFound, "File not found")
		return "", false
	}

	return filePath, true
}

// readFileContent reads file content from vault
// Returns content and size if successful, writes error and returns false if failed
func (s *Server) readFileContent(w http.ResponseWriter, v *vault.Vault, filePath string) (string, int64, bool) {
	content, size, err := s.readVaultFile(v, filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return "", 0, false
	}

	return content, size, true
}

// readFileContentBinary reads file content from vault as binary
// Returns content bytes and size if successful, writes error and returns false if failed
func (s *Server) readFileContentBinary(w http.ResponseWriter, v *vault.Vault, filePath string) ([]byte, int64, bool) {
	content, size, err := s.readVaultFileInBinary(v, filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("File not found: %v", err))
		return nil, 0, false
	}

	return content, size, true
}
