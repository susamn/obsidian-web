package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/vault"
	"github.com/susamn/obsidian-web/internal/web"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	log.Println("Loading configuration...")
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Ensure all required directories exist
	if err := cfg.EnsureDirectories(); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Display loaded configuration
	log.Printf("\n=== Obsidian Web Configuration ===")
	log.Printf("Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Database: %s", cfg.Database.Path)
	log.Printf("Log Level: %s", cfg.Logging.Level)

	// Display configured vaults
	enabledVaults := cfg.ListEnabledVaults()
	log.Printf("\n=== Configured Vaults: %d ===", len(enabledVaults))
	for _, vaultCfg := range enabledVaults {
		log.Printf("  - %s (%s) [%s]", vaultCfg.Name, vaultCfg.ID, vaultCfg.Storage.GetType())
	}

	// Create application context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize vaults
	log.Printf("\n=== Initializing Vaults ===")
	vaults := make(map[string]*vault.Vault)

	for _, vaultCfg := range enabledVaults {
		log.Printf("Creating vault: %s", vaultCfg.Name)

		v, err := vault.NewVault(ctx, &vaultCfg)
		if err != nil {
			log.Printf("Failed to create vault %s: %v", vaultCfg.ID, err)
			continue
		}

		// Start vault
		log.Printf("Starting vault: %s", vaultCfg.Name)
		if err := v.Start(); err != nil {
			log.Printf("Failed to start vault %s: %v", vaultCfg.ID, err)
			continue
		}

		vaults[vaultCfg.ID] = v
		log.Printf("✓ Vault %s started successfully", vaultCfg.Name)
	}

	if len(vaults) == 0 {
		log.Fatal("No vaults were successfully initialized")
	}

	log.Printf("Successfully initialized %d vault(s)", len(vaults))

	// Wait for vaults to be ready
	log.Printf("\n=== Waiting for vaults to be ready ===")
	for id, v := range vaults {
		if err := v.WaitForReady(30 * time.Second); err != nil {
			log.Printf("Warning: Vault %s not ready: %v", id, err)
		} else {
			log.Printf("✓ Vault %s is ready", id)
		}
	}

	// Create and start web server
	log.Printf("\n=== Starting Web Server ===")
	server := web.NewServer(ctx, cfg, vaults)

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}

	log.Printf("✓ Web server started on %s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("\n=== Obsidian Web is running ===")
	log.Printf("API available at: http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Health check: http://%s:%d/api/v1/health", cfg.Server.Host, cfg.Server.Port)
	log.Printf("\nPress Ctrl+C to stop")

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal
	<-sigChan

	log.Printf("\n=== Shutting down gracefully ===")

	// Stop web server
	log.Println("Stopping web server...")
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping web server: %v", err)
	}

	// Stop all vaults
	log.Println("Stopping vaults...")
	for id, v := range vaults {
		log.Printf("Stopping vault: %s", id)
		if err := v.Stop(); err != nil {
			log.Printf("Error stopping vault %s: %v", id, err)
		}
	}

	log.Println("✓ Shutdown complete")
}
