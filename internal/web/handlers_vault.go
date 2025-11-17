package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/susamn/obsidian-web/internal/vault"
)

// VaultInfo represents vault information
type VaultInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Active bool   `json:"active"`
}

// VaultsResponse represents list of vaults
type VaultsResponse struct {
	Vaults []VaultInfo `json:"vaults"`
	Total  int         `json:"total"`
}

// handleVaults godoc
// @Summary List all vaults
// @Description Get a list of all available vaults
// @Tags vaults
// @Produce json
// @Success 200 {object} VaultsResponse
// @Failure 405 {object} ErrorResponse
// @Router /api/v1/vaults [get]
func (s *Server) handleVaults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	vaults := s.listVaults()
	vaultInfos := make([]VaultInfo, 0, len(vaults))

	for _, v := range vaults {
		vaultInfos = append(vaultInfos, VaultInfo{
			ID:     v.VaultID(),
			Name:   v.VaultName(),
			Status: v.GetStatus().String(),
			Active: v.IsActive(),
		})
	}

	writeSuccess(w, VaultsResponse{
		Vaults: vaultInfos,
		Total:  len(vaultInfos),
	})
}

// handleVaultOps handles vault operations (GET info, POST start/stop/resume)
func (s *Server) handleVaultOps(w http.ResponseWriter, r *http.Request) {
	// Extract vault ID and operation from path
	// Format: /api/v1/vaults/:id or /api/v1/vaults/:id/:operation
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/vaults/")
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "Vault ID required")
		return
	}

	vaultID := parts[0]

	// Get vault info (GET /api/v1/vaults/:id)
	if r.Method == http.MethodGet && len(parts) == 1 {
		s.handleGetVaultInfo(w, r, vaultID)
		return
	}

	// Vault operations (POST /api/v1/vaults/:id/:operation)
	if r.Method == http.MethodPost && len(parts) == 2 {
		operation := parts[1]
		s.handleVaultOperation(w, r, vaultID, operation)
		return
	}

	writeError(w, http.StatusBadRequest, "Invalid request")
}

// handleGetVaultInfo godoc
// @Summary Get vault info
// @Description Get information about a specific vault
// @Tags vaults
// @Produce json
// @Param id path string true "Vault ID"
// @Success 200 {object} object "Vault information"
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/vaults/{id} [get]
func (s *Server) handleGetVaultInfo(w http.ResponseWriter, r *http.Request, vaultID string) {
	v, ok := s.getVault(vaultID)
	if !ok {
		writeError(w, http.StatusNotFound, "Vault not found")
		return
	}

	metrics := v.GetMetrics()

	info := map[string]interface{}{
		"id":                metrics.VaultID,
		"name":              metrics.VaultName,
		"status":            metrics.Status.String(),
		"active":            v.IsActive(),
		"uptime":            metrics.Uptime.String(),
		"indexed_files":     metrics.IndexedFiles,
		"recent_operations": metrics.RecentOperations,
	}

	writeSuccess(w, info)
}

// handleVaultOperation godoc
// @Summary Perform a vault operation
// @Description Perform an operation on a vault (start, stop, resume)
// @Tags vaults
// @Produce json
// @Param id path string true "Vault ID"
// @Param operation path string true "Operation (start, stop, resume)"
// @Success 200 {object} object "Operation status"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/vaults/{id}/{operation} [post]
func (s *Server) handleVaultOperation(w http.ResponseWriter, r *http.Request, vaultID, operation string) {
	v, ok := s.getVault(vaultID)
	if !ok {
		writeError(w, http.StatusNotFound, "Vault not found")
		return
	}

	var err error

	switch operation {
	case "start":
		err = s.startVault(v)
	case "stop":
		err = s.stopVault(v)
	case "resume":
		err = s.resumeVault(v)
	default:
		writeError(w, http.StatusBadRequest, "Invalid operation")
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Operation failed: %v", err))
		return
	}

	writeSuccess(w, map[string]string{
		"vault":     vaultID,
		"operation": operation,
		"status":    "success",
	})
}

// startVault starts a vault
func (s *Server) startVault(v *vault.Vault) error {
	if v.GetStatus() == vault.VaultStatusActive {
		return fmt.Errorf("vault already active")
	}

	if v.GetStatus() == vault.VaultStatusStopped {
		return v.Resume()
	}

	return v.Start()
}

// stopVault stops a vault
func (s *Server) stopVault(v *vault.Vault) error {
	if v.GetStatus() == vault.VaultStatusStopped {
		return nil // Already stopped
	}

	return v.Stop()
}

// resumeVault resumes a stopped vault
func (s *Server) resumeVault(v *vault.Vault) error {
	if v.GetStatus() != vault.VaultStatusStopped {
		return fmt.Errorf("vault is not stopped")
	}

	return v.Resume()
}
