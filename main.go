package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/susamn/obsidian-web/internal/config"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	fmt.Println("Loading configuration...")
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
	fmt.Printf("\n=== Obsidian Web Configuration ===\n")
	fmt.Printf("Server: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Database: %s\n", cfg.Database.Path)
	fmt.Printf("Log Level: %s\n", cfg.Logging.Level)

	// Display configured vaults
	fmt.Printf("\n=== Configured Vaults ===\n")
	for _, vault := range cfg.ListEnabledVaults() {
		fmt.Printf("\nVault: %s (%s)\n", vault.Name, vault.ID)
		fmt.Printf("  Default: %v\n", vault.Default)
		fmt.Printf("  Storage Type: %s\n", vault.Storage.GetType())
		fmt.Printf("  Index Path: %s\n", vault.IndexPath)
	}

	fmt.Printf("\n=== Initializing Services ===\n")

	// Initialize indexing service for default vault
	defaultVault := cfg.GetDefaultVault()
	if defaultVault != nil {
		fmt.Printf("\nInitializing index service for: %s\n", defaultVault.Name)

		// Note: For now, we're just demonstrating the integration
		// In production, this would run in the background
		// indexService, err := indexing.NewIndexService(defaultVault)
		// if err != nil {
		//     log.Printf("Failed to create index service: %v", err)
		// } else {
		//     defer indexService.Close()
		//     if err := indexService.Index(); err != nil {
		//         log.Printf("Failed to index vault: %v", err)
		//     }
		// }

		fmt.Printf("Index path: %s\n", defaultVault.IndexPath)
		fmt.Printf("Storage type: %s\n", defaultVault.Storage.GetType())
	}

	fmt.Printf("\n=== Ready to start services ===\n")

	// TODO: Complete initialization
	// - Database module
	// - Sync service (will handle storage configs and trigger re-indexing)
	// - HTTP server for API endpoints
	// - Search service

	os.Exit(0)
}
