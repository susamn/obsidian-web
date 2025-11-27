package web

import (
	"context"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/vault"
)

func TestNewServer(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 19878,
		},
	}

	vaults := make(map[string]*vault.Vault)

	server := NewServer(ctx, cfg, vaults)

	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	if server.config != cfg {
		t.Error("Config not set correctly")
	}

	if server.server == nil {
		t.Error("HTTP server not created")
	}

	expectedAddr := "localhost:8080"
	if server.server.Addr != expectedAddr {
		t.Errorf("Expected addr %s, got %s", expectedAddr, server.server.Addr)
	}
}

func TestServer_StartStop(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 0, // Random port
		},
	}

	vaults := make(map[string]*vault.Vault)
	server := NewServer(ctx, cfg, vaults)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Try to start again (should fail)
	if err := server.Start(); err == nil {
		t.Error("Expected error when starting already-started server")
	}

	// Stop server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Stop again (should be idempotent)
	if err := server.Stop(); err != nil {
		t.Error("Stop should be idempotent")
	}
}

func TestServer_GetVault(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 19878,
		},
	}

	tempDir := t.TempDir()
	indexDir := t.TempDir()

	vaultCfg := &config.VaultConfig{
		ID:        "test-vault",
		Name:      "Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	v, err := vault.NewVault(ctx, vaultCfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	vaults := map[string]*vault.Vault{
		"test-vault": v,
	}

	server := NewServer(ctx, cfg, vaults)

	// Test getVault
	retrieved, ok := server.getVault("test-vault")
	if !ok {
		t.Error("Expected to find vault")
	}

	if retrieved != v {
		t.Error("Retrieved vault doesn't match")
	}

	// Test non-existent vault
	_, ok = server.getVault("non-existent")
	if ok {
		t.Error("Expected to not find non-existent vault")
	}
}

func TestServer_ListVaults(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 19878,
		},
	}

	tempDir1 := t.TempDir()
	indexDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	indexDir2 := t.TempDir()

	vaultCfg1 := &config.VaultConfig{
		ID:        "vault-1",
		Name:      "Vault 1",
		Enabled:   true,
		IndexPath: indexDir1 + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir1,
			},
		},
	}

	vaultCfg2 := &config.VaultConfig{
		ID:        "vault-2",
		Name:      "Vault 2",
		Enabled:   true,
		IndexPath: indexDir2 + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir2,
			},
		},
	}

	v1, _ := vault.NewVault(ctx, vaultCfg1)
	v2, _ := vault.NewVault(ctx, vaultCfg2)

	vaults := map[string]*vault.Vault{
		"vault-1": v1,
		"vault-2": v2,
	}

	server := NewServer(ctx, cfg, vaults)

	// Test listVaults
	list := server.listVaults()

	if len(list) != 2 {
		t.Errorf("Expected 2 vaults, got %d", len(list))
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	t.Skip("Skipping integration test - tested in handler tests")
}
