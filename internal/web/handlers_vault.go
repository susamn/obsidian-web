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

// handleVaults handles GET /api/v1/vaults
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

// handleGetVaultInfo handles GET /api/v1/vaults/:id
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

// handleVaultOperation handles POST /api/v1/vaults/:id/:operation
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
