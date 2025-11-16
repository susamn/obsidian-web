package vault

import (
	"context"
	"testing"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
)

// TestNewVault_ValidConfig tests creating a vault with valid configuration
func TestNewVault_ValidConfig(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	cfg := &config.VaultConfig{
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

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if vault == nil {
		t.Fatal("Vault is nil")
	}

	if vault.config.ID != "test-vault" {
		t.Errorf("Expected vault ID 'test-vault', got '%s'", vault.config.ID)
	}

	if vault.status != VaultStatusInitializing {
		t.Errorf("Expected status Initializing, got %s", vault.status)
	}

	if vault.vaultPath != tempDir {
		t.Errorf("Expected vault path '%s', got '%s'", tempDir, vault.vaultPath)
	}

	// Verify services were created
	if vault.syncService == nil {
		t.Error("Sync service is nil")
	}

	if vault.indexService == nil {
		t.Error("Index service is nil")
	}

	// Note: Index is created when the service starts, not when vault is created
	// Before Start(), GetIndex() may return nil
}

// TestNewVault_NilConfig tests creating vault with nil config
func TestNewVault_NilConfig(t *testing.T) {
	ctx := context.Background()

	vault, err := NewVault(ctx, nil)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}

	if vault != nil {
		t.Error("Expected nil vault for nil config")
	}
}

// TestNewVault_DisabledVault tests creating vault with disabled flag
func TestNewVault_DisabledVault(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	cfg := &config.VaultConfig{
		ID:      "disabled-vault",
		Name:    "Disabled Vault",
		Enabled: false, // Disabled
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	vault, err := NewVault(ctx, cfg)
	if err == nil {
		t.Fatal("Expected error for disabled vault, got nil")
	}

	if vault != nil {
		t.Error("Expected nil vault for disabled config")
	}
}

// TestNewVault_InvalidStorage tests creating vault with invalid storage config
func TestNewVault_InvalidStorage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		storage config.StorageConfig
	}{
		{
			name: "EmptyLocalPath",
			storage: config.StorageConfig{
				Type:  "local",
				Local: &config.LocalStorageConfig{Path: ""},
			},
		},
		{
			name: "NilLocalConfig",
			storage: config.StorageConfig{
				Type:  "local",
				Local: nil,
			},
		},
		{
			name: "S3NotImplemented",
			storage: config.StorageConfig{
				Type: "s3",
				S3: &config.S3StorageConfig{
					Bucket: "test-bucket",
					Region: "us-east-1",
				},
			},
		},
		{
			name: "MinIONotImplemented",
			storage: config.StorageConfig{
				Type: "minio",
				MinIO: &config.MinIOStorageConfig{
					Endpoint: "localhost:9000",
					Bucket:   "test-bucket",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.VaultConfig{
				ID:      "test-vault",
				Name:    "Test Vault",
				Enabled: true,
				Storage: tt.storage,
			}

			vault, err := NewVault(ctx, cfg)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tt.name)
			}

			if vault != nil {
				t.Errorf("Expected nil vault for %s", tt.name)
			}
		})
	}
}

// TestVault_Lifecycle tests the full lifecycle of Start -> Stop
func TestVault_Lifecycle(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	cfg := &config.VaultConfig{
		ID:        "lifecycle-vault",
		Name:      "Lifecycle Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	// Initial status should be Initializing
	if vault.GetStatus() != VaultStatusInitializing {
		t.Errorf("Expected Initializing status, got %s", vault.GetStatus())
	}

	// Start vault
	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	// Status should transition to Starting
	status := vault.GetStatus()
	if status != VaultStatusInitializing && status != VaultStatusActive {
		t.Errorf("Expected Starting or Ready status after Start(), got %s", status)
	}

	// Wait for vault to be ready (with timeout)
	if err := vault.WaitForReady(10 * time.Second); err != nil {
		t.Fatalf("Vault did not become ready: %v", err)
	}

	// Should be ready now
	if !vault.IsReady() {
		t.Error("Vault should be ready")
	}

	if vault.GetStatus() != VaultStatusActive {
		t.Errorf("Expected Ready status, got %s", vault.GetStatus())
	}

	// Index should be available for searching
	if vault.GetIndex() == nil {
		t.Error("Index should be available after vault is ready")
	}

	// Stop vault
	if err := vault.Stop(); err != nil {
		t.Fatalf("Failed to stop vault: %v", err)
	}

	// Should be stopped
	if vault.GetStatus() != VaultStatusStopped {
		t.Errorf("Expected Stopped status, got %s", vault.GetStatus())
	}

	// Calling Stop again should not error
	if err := vault.Stop(); err != nil {
		t.Error("Stop() should be idempotent")
	}
}

// TestVault_StartTwice tests that starting vault twice returns error
func TestVault_StartTwice(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	cfg := &config.VaultConfig{
		ID:        "double-start-vault",
		Name:      "Double Start Test",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	// First start should succeed
	if err := vault.Start(); err != nil {
		t.Fatalf("First Start() failed: %v", err)
	}

	// Second start should fail
	err = vault.Start()
	if err == nil {
		t.Error("Expected error when starting vault twice, got nil")
	}
}

// TestVault_WaitForReady_Timeout tests WaitForReady with timeout
func TestVault_WaitForReady_Timeout(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	cfg := &config.VaultConfig{
		ID:        "timeout-vault",
		Name:      "Timeout Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	// Don't start the vault, WaitForReady should timeout
	err = vault.WaitForReady(100 * time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

// TestVault_WaitForReady_AfterStop tests WaitForReady on stopped vault
func TestVault_WaitForReady_AfterStop(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	cfg := &config.VaultConfig{
		ID:        "stopped-vault",
		Name:      "Stopped Vault Test",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	// Manually set status to stopped
	vault.setStatus(VaultStatusStopped)

	// WaitForReady should return error
	err = vault.WaitForReady(1 * time.Second)
	if err == nil {
		t.Error("Expected error for stopped vault, got nil")
	}
}

// TestVault_GetMetrics tests metrics collection
func TestVault_GetMetrics(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	cfg := &config.VaultConfig{
		ID:        "metrics-vault",
		Name:      "Metrics Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	// Get metrics before starting
	metrics := vault.GetMetrics()

	if metrics.VaultID != "metrics-vault" {
		t.Errorf("Expected VaultID 'metrics-vault', got '%s'", metrics.VaultID)
	}

	if metrics.VaultName != "Metrics Test Vault" {
		t.Errorf("Expected VaultName 'Metrics Test Vault', got '%s'", metrics.VaultName)
	}

	if metrics.Status != VaultStatusInitializing {
		t.Errorf("Expected status Initializing, got %s", metrics.Status)
	}

	if metrics.Uptime != 0 {
		t.Error("Expected uptime 0 before starting")
	}

	// Start vault
	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	// Wait for ready
	vault.WaitForReady(10 * time.Second)

	// Get metrics after starting
	metrics = vault.GetMetrics()

	if metrics.Status != VaultStatusActive {
		t.Errorf("Expected status Ready, got %s", metrics.Status)
	}

	if metrics.Uptime == 0 {
		t.Error("Expected non-zero uptime after starting")
	}
}

// TestVault_GetServices tests service accessor methods
func TestVault_GetServices(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	cfg := &config.VaultConfig{
		ID:        "services-vault",
		Name:      "Services Test Vault",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	// Test GetSyncService
	syncSvc := vault.GetSyncService()
	if syncSvc == nil {
		t.Error("GetSyncService() returned nil")
	}

	// Test GetIndexService
	indexSvc := vault.GetIndexService()
	if indexSvc == nil {
		t.Error("GetIndexService() returned nil")
	}

	// Test GetIndex (may be nil before Start)
	// Index is created when the service starts
	_ = vault.GetIndex()

	// Test VaultID
	if vault.VaultID() != "services-vault" {
		t.Errorf("Expected VaultID 'services-vault', got '%s'", vault.VaultID())
	}

	// Test VaultName
	if vault.VaultName() != "Services Test Vault" {
		t.Errorf("Expected VaultName 'Services Test Vault', got '%s'", vault.VaultName())
	}
}

// TestVault_VaultStatus_String tests VaultStatus string conversion
func TestVault_VaultStatus_String(t *testing.T) {
	tests := []struct {
		status   VaultStatus
		expected string
	}{
		{VaultStatusInitializing, "initializing"},
		{VaultStatusActive, "active"},
		{VaultStatusStopped, "stopped"},
		{VaultStatusError, "error"},
		{VaultStatus(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.status.String()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestGetVaultPath tests vault path determination for different storage types
func TestGetVaultPath(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *config.VaultConfig
		expectError  bool
		expectedPath string
	}{
		{
			name: "LocalStorage",
			cfg: &config.VaultConfig{
				ID: "local-vault",
				Storage: config.StorageConfig{
					Type: "local",
					Local: &config.LocalStorageConfig{
						Path: "/vault/data",
					},
				},
			},
			expectError:  false,
			expectedPath: "/vault/data",
		},
		{
			name: "LocalStorage_EmptyPath",
			cfg: &config.VaultConfig{
				ID: "empty-path-vault",
				Storage: config.StorageConfig{
					Type: "local",
					Local: &config.LocalStorageConfig{
						Path: "",
					},
				},
			},
			expectError: true,
		},
		{
			name: "S3Storage_NotImplemented",
			cfg: &config.VaultConfig{
				ID: "s3-vault",
				Storage: config.StorageConfig{
					Type: "s3",
					S3: &config.S3StorageConfig{
						Bucket: "test-bucket",
					},
				},
			},
			expectError:  true,
			expectedPath: "/tmp/vault-cache/s3-vault",
		},
		{
			name: "MinIOStorage_NotImplemented",
			cfg: &config.VaultConfig{
				ID: "minio-vault",
				Storage: config.StorageConfig{
					Type: "minio",
					MinIO: &config.MinIOStorageConfig{
						Bucket: "test-bucket",
					},
				},
			},
			expectError:  true,
			expectedPath: "/tmp/vault-cache/minio-vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := getVaultPath(tt.cfg)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if tt.expectedPath != "" && path != tt.expectedPath {
				t.Errorf("Expected path '%s', got '%s'", tt.expectedPath, path)
			}
		})
	}
}

// TestVault_ContextCancellation tests vault behavior when context is cancelled
func TestVault_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	cfg := &config.VaultConfig{
		ID:        "cancel-vault",
		Name:      "Context Cancel Test",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	// Cancel context
	cancel()

	// Give it a moment to process cancellation
	time.Sleep(100 * time.Millisecond)

	// Stop should still work
	if err := vault.Stop(); err != nil {
		t.Errorf("Stop() failed after context cancellation: %v", err)
	}
}

// TestVault_ConcurrentAccess tests concurrent access to vault methods
func TestVault_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	indexDir := t.TempDir()

	cfg := &config.VaultConfig{
		ID:        "concurrent-vault",
		Name:      "Concurrent Access Test",
		Enabled:   true,
		IndexPath: indexDir + "/test.bleve",
		Storage: config.StorageConfig{
			Type: "local",
			Local: &config.LocalStorageConfig{
				Path: tempDir,
			},
		},
	}

	vault, err := NewVault(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create vault: %v", err)
	}

	if err := vault.Start(); err != nil {
		t.Fatalf("Failed to start vault: %v", err)
	}

	// Wait for ready
	vault.WaitForReady(10 * time.Second)

	// Concurrent reads (should not cause race conditions)
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				_ = vault.GetStatus()
				_ = vault.GetMetrics()
				_ = vault.GetSyncService()
				_ = vault.GetIndexService()
				_ = vault.GetIndex()
				_ = vault.IsReady()
				_ = vault.VaultID()
				_ = vault.VaultName()
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Stop vault
	if err := vault.Stop(); err != nil {
		t.Fatalf("Failed to stop vault: %v", err)
	}
}
