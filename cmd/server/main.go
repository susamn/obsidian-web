package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/vault"
	"github.com/susamn/obsidian-web/internal/web"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration (using basic logging before logger is initialized)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	if err := logger.Initialize(&cfg.Logging); err != nil {
		logger.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Info("Loading configuration...")

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Fatalf("Configuration validation failed: %v", err)
	}

	// Ensure all required directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		logger.Fatalf("Failed to create directories: %v", err)
	}

	// Display loaded configuration
	logger.Info("\n=== Obsidian Web Configuration ===")
	logger.Infof("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Infof("Database: %s", cfg.Database.Path)
	logger.Infof("Log Level: %s", cfg.Logging.Level)

	// Display configured vaults
	enabledVaults := cfg.ListEnabledVaults()
	logger.Infof("\n=== Configured Vaults: %d ===", len(enabledVaults))
	for _, vaultCfg := range enabledVaults {
		logger.Infof("  - %s (%s) [%s]", vaultCfg.Name, vaultCfg.ID, vaultCfg.Storage.GetType())
	}

	// Create application context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize vaults
	logger.Info("\n=== Initializing Vaults ===")
	vaults := make(map[string]*vault.Vault)

	for _, vaultCfg := range enabledVaults {
		logger.WithField("vault", vaultCfg.Name).Info("Creating vault")

		v, err := vault.NewVault(ctx, &vaultCfg)
		if err != nil {
			logger.WithError(err).WithField("vault_id", vaultCfg.ID).Error("Failed to create vault")
			continue
		}

		// Start vault
		logger.WithField("vault", vaultCfg.Name).Info("Starting vault")
		if err := v.Start(); err != nil {
			logger.WithError(err).WithField("vault_id", vaultCfg.ID).Error("Failed to start vault")
			continue
		}

		vaults[vaultCfg.ID] = v
		logger.WithField("vault", vaultCfg.Name).Info("✓ Vault started successfully")
	}

	if len(vaults) == 0 {
		logger.Fatal("No vaults were successfully initialized")
	}

	logger.Infof("Successfully initialized %d vault(s)", len(vaults))

	// Wait for vaults to be ready
	logger.Info("\n=== Waiting for vaults to be ready ===")
	for id, v := range vaults {
		if err := v.WaitForReady(30 * time.Second); err != nil {
			logger.WithError(err).WithField("vault_id", id).Warn("Vault not ready")
		} else {
			logger.WithField("vault_id", id).Info("✓ Vault is ready")
		}
	}

	// Create and start web server
	logger.Info("\n=== Starting Web Server ===")
	server := web.NewServer(ctx, cfg, vaults)

	if err := server.Start(); err != nil {
		logger.WithError(err).Fatal("Failed to start web server")
	}

	logger.WithFields(map[string]interface{}{
		"host": cfg.Server.Host,
		"port": cfg.Server.Port,
	}).Info("✓ Web server started")
	logger.Info("\n=== Obsidian Web is running ===")
	logger.Infof("API available at: http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Infof("Health check: http://%s:%d/api/v1/health", cfg.Server.Host, cfg.Server.Port)
	logger.Info("\nPress Ctrl+C to stop")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-sigChan

	logger.Info("\n=== Shutting down gracefully ===")

	// Stop web server
	logger.Info("Stopping web server...")
	if err := server.Stop(); err != nil {
		logger.WithError(err).Error("Error stopping web server")
	}

	// Stop all vaults
	logger.Info("Stopping vaults...")
	for id, v := range vaults {
		logger.WithField("vault_id", id).Info("Stopping vault")
		if err := v.Stop(); err != nil {
			logger.WithError(err).WithField("vault_id", id).Error("Error stopping vault")
		}
	}

	logger.Info("✓ Shutdown complete")
}
