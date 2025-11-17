package web

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/vault"
)

// Server represents the HTTP server
type Server struct {
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	config  *config.Config
	vaults  map[string]*vault.Vault
	server  *http.Server
	started bool
}

// NewServer creates a new HTTP server
func NewServer(ctx context.Context, cfg *config.Config, vaults map[string]*vault.Vault) *Server {
	serverCtx, cancel := context.WithCancel(ctx)

	s := &Server{
		ctx:    serverCtx,
		cancel: cancel,
		config: cfg,
		vaults: vaults,
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
	// API routes
	mux.HandleFunc("/api/v1/files/", s.handleGetFile)
	mux.HandleFunc("/api/v1/raw/", s.handleGetRaw)
	mux.HandleFunc("/api/v1/search/", s.handleSearch)
	mux.HandleFunc("/api/v1/vaults", s.handleVaults)
	mux.HandleFunc("/api/v1/vaults/", s.handleVaultOps)
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/metrics/", s.handleMetrics)

	// Swagger
	mux.HandleFunc("/swagger/", s.handleSwagger)

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
