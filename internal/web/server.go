package web

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/sse"
	"github.com/susamn/obsidian-web/internal/vault"
)

// Server represents the HTTP server
type Server struct {
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	config     *config.Config
	vaults     map[string]*vault.Vault
	server     *http.Server
	sseManager *sse.Manager
	started    bool
}

// NewServer creates a new HTTP server
func NewServer(ctx context.Context, cfg *config.Config, vaults map[string]*vault.Vault) *Server {
	serverCtx, cancel := context.WithCancel(ctx)

	// Create SSE manager
	sseManager := sse.NewManager(serverCtx)

	s := &Server{
		ctx:        serverCtx,
		cancel:     cancel,
		config:     cfg,
		vaults:     vaults,
		sseManager: sseManager,
	}

	// Setup routes
	mux := http.NewServeMux()
	s.setupRoutes(mux)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.withMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes(mux *http.ServeMux) {
	// API routes - ID-based endpoints must come before path-based ones

	// ACTIVE ROUTES - Used by frontend
	mux.HandleFunc("/api/v1/assets/", s.handleGetAsset)                     // Images/assets in markdown
	mux.HandleFunc("/api/v1/file/create", s.handleCreateFile)               // CreateNoteDialog
	mux.HandleFunc("/api/v1/files/sr/by-id/", s.handleStructuredRenderByID) // StructuredRenderer
	mux.HandleFunc("/api/v1/files/by-id/", s.handleGetFileByID)             // fileService.getFileContent
	mux.HandleFunc("/api/v1/files/tree/", s.handleGetTree)                  // fileService.getTree
	mux.HandleFunc("/api/v1/files/meta/", s.handleGetMetadata)              // fileService.getMetadata
	mux.HandleFunc("/api/v1/search/", s.handleSearch)                       // SearchPanel
	mux.HandleFunc("/api/v1/vaults", s.handleVaults)                        // HomeView
	mux.HandleFunc("/api/v1/health", s.handleHealth)                        // Health check
	mux.HandleFunc("/api/v1/sse/", s.handleSSE)                             // useSSE composable

	mux.HandleFunc("/api/v1/files/reindex/", s.handleForceReindex) // Force reindex
	mux.HandleFunc("/api/v1/vaults/", s.handleVaultOps)            // Vault operations
	mux.HandleFunc("/api/v1/sse/stats", s.handleSSEStats)          // SSE stats
	mux.HandleFunc("/swagger/", s.handleSwagger)                   // Swagger UI

	// COMMENTED OUT - UNUSED ROUTES
	// mux.HandleFunc("/api/v1/files/tree-by-id/", s.handleGetTreeByID)        // Lazy-loaded tree by ID
	// mux.HandleFunc("/api/v1/files/children-by-id/", s.handleGetChildrenByID) // Lazy-loaded children by ID
	// mux.HandleFunc("/api/v1/files/ssr/by-id/", s.handleSSRFileByID)         // SSR renderer (removed from frontend)
	// mux.HandleFunc("/api/v1/files/reindex/", s.handleForceReindex)          // Force reindex
	// mux.HandleFunc("/api/v1/files/children/", s.handleGetChildren)          // Lazy-loaded children (removed)
	// mux.HandleFunc("/api/v1/files/refresh/", s.handleRefreshTree)           // Manual tree refresh (removed)
	// mux.HandleFunc("/api/v1/files/", s.handleGetFile)                       // Path-based file access
	// mux.HandleFunc("/api/v1/raw/", s.handleGetRaw)                          // Raw file serving
	// mux.HandleFunc("/api/v1/vaults/", s.handleVaultOps)                     // Vault operations
	// mux.HandleFunc("/api/v1/metrics/", s.handleMetrics)                     // Metrics endpoint
	// mux.HandleFunc("/api/v1/sse/stats", s.handleSSEStats)                   // SSE stats
	// mux.HandleFunc("/swagger/", s.handleSwagger)                            // Swagger UI

	// Static files
	spa := spaHandler{staticPath: "./internal/public", indexPath: "index.html"}
	mux.Handle("/", spa)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}
	s.started = true
	s.mu.Unlock()

	// Start SSE manager
	s.sseManager.Start()

	logger.WithField("address", s.server.Addr).Info("Starting HTTP server")

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Check for immediate errors
	select {
	case err := <-errChan:
		s.mu.Lock()
		s.started = false
		s.mu.Unlock()
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	logger.Info("Stopping HTTP server")

	// Stop SSE manager first
	s.sseManager.Stop()

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown server
	if err := s.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.cancel()

	s.mu.Lock()
	s.started = false
	s.mu.Unlock()

	logger.Info("HTTP server stopped")
	return nil
}

// getVault retrieves a vault by ID (thread-safe)
func (s *Server) getVault(vaultID string) (*vault.Vault, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vaults[vaultID]
	return v, ok
}

// listVaults returns all vaults (thread-safe)
func (s *Server) listVaults() []*vault.Vault {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vaults := make([]*vault.Vault, 0, len(s.vaults))
	for _, v := range s.vaults {
		vaults = append(vaults, v)
	}
	return vaults
}

// GetSSEManager returns the SSE manager
func (s *Server) GetSSEManager() *sse.Manager {
	return s.sseManager
}
