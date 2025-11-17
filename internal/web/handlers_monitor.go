package web

import (
	"net/http"
	"runtime"
	"time"
)

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
}

// MetricsResponse represents vault metrics response
type MetricsResponse struct {
	VaultID          string                   `json:"vault_id"`
	VaultName        string                   `json:"vault_name"`
	Status           string                   `json:"status"`
	Active           bool                     `json:"active"`
	Uptime           string                   `json:"uptime"`
	IndexedFiles     uint64                   `json:"indexed_files"`
	RecentOperations []map[string]interface{} `json:"recent_operations"`
}

// SystemMetrics represents system-wide metrics
type SystemMetrics struct {
	GoVersion    string `json:"go_version"`
	NumGoroutine int    `json:"num_goroutine"`
	MemAllocMB   uint64 `json:"mem_alloc_mb"`
	MemTotalMB   uint64 `json:"mem_total_mb"`
	NumCPU       int    `json:"num_cpu"`
}

var serverStartTime = time.Now()

// handleHealth handles GET /api/v1/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	writeSuccess(w, HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Uptime:    time.Since(serverStartTime).String(),
	})
}

// handleMetrics handles GET /api/v1/metrics/:vault
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID
	vaultID := s.extractVaultID(r.URL.Path, "/api/v1/metrics/")

	// If no vault ID, return system metrics
	if vaultID == "" {
		s.handleSystemMetrics(w, r)
		return
	}

	// Get vault-specific metrics
	v, ok := s.getVault(vaultID)
	if !ok {
		writeError(w, http.StatusNotFound, "Vault not found")
		return
	}

	metrics := v.GetMetrics()

	// Convert recent operations to map format
	recentOps := make([]map[string]interface{}, 0, len(metrics.RecentOperations))
	for _, op := range metrics.RecentOperations {
		recentOps = append(recentOps, map[string]interface{}{
			"path":      op.Path,
			"operation": op.Operation,
			"timestamp": op.Timestamp,
		})
	}

	response := MetricsResponse{
		VaultID:          metrics.VaultID,
		VaultName:        metrics.VaultName,
		Status:           metrics.Status.String(),
		Active:           v.IsActive(),
		Uptime:           metrics.Uptime.String(),
		IndexedFiles:     metrics.IndexedFiles,
		RecentOperations: recentOps,
	}

	writeSuccess(w, response)
}

// handleSystemMetrics returns system-wide metrics
func (s *Server) handleSystemMetrics(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	sysMetrics := SystemMetrics{
		GoVersion:    runtime.Version(),
		NumGoroutine: runtime.NumGoroutine(),
		MemAllocMB:   m.Alloc / 1024 / 1024,
		MemTotalMB:   m.TotalAlloc / 1024 / 1024,
		NumCPU:       runtime.NumCPU(),
	}

	// Get all vault statuses
	vaults := s.listVaults()
	vaultStatuses := make([]VaultInfo, 0, len(vaults))
	for _, v := range vaults {
		vaultStatuses = append(vaultStatuses, VaultInfo{
			ID:     v.VaultID(),
			Name:   v.VaultName(),
			Status: v.GetStatus().String(),
			Active: v.IsActive(),
		})
	}

	response := map[string]interface{}{
		"system": sysMetrics,
		"vaults": vaultStatuses,
		"uptime": time.Since(serverStartTime).String(),
	}

	writeSuccess(w, response)
}
