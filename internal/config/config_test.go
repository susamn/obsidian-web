package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Server defaults
	if cfg.Server.Host != "localhost" {
		t.Errorf("Expected default host 'localhost', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected default read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}

	// Logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.Logging.Level)
	}

	// Vault defaults
	if len(cfg.Vaults) == 0 {
		t.Error("Default config should have at least one vault")
	}
	if !cfg.Vaults[0].Default {
		t.Error("First vault should be marked as default")
	}

	// Search defaults
	if cfg.Search.DefaultLimit != 20 {
		t.Errorf("Expected default search limit 20, got %d", cfg.Search.DefaultLimit)
	}

	// Indexing defaults
	if cfg.Indexing.BatchSize != 100 {
		t.Errorf("Expected default batch size 100, got %d", cfg.Indexing.BatchSize)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 9090
  read_timeout: 60s
  write_timeout: 60s

logging:
  level: "debug"
  format: "json"
  output: "stderr"

vaults:
  - id: "test"
    name: "Test Vault"
    storage:
      type: "local"
      local:
        path: "/tmp/vault"
    index_path: "/tmp/indexes/test"
    db_path: "/tmp/db/vault.db"
    enabled: true
    default: true

search:
  default_limit: 50
  max_limit: 200
  cache_size_mb: 1024

indexing:
  batch_size: 200
  auto_index_on_startup: false
  watch_for_changes: true
  update_interval: 10m
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected host '0.0.0.0', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 60*time.Second {
		t.Errorf("Expected read timeout 60s, got %v", cfg.Server.ReadTimeout)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Expected log format 'json', got '%s'", cfg.Logging.Format)
	}

	if len(cfg.Vaults) != 1 {
		t.Errorf("Expected 1 vault, got %d", len(cfg.Vaults))
	}
	if cfg.Vaults[0].ID != "test" {
		t.Errorf("Expected vault ID 'test', got '%s'", cfg.Vaults[0].ID)
	}

	if cfg.Search.DefaultLimit != 50 {
		t.Errorf("Expected default limit 50, got %d", cfg.Search.DefaultLimit)
	}

	if cfg.Indexing.BatchSize != 200 {
		t.Errorf("Expected batch size 200, got %d", cfg.Indexing.BatchSize)
	}
}

func TestEnvOverrides(t *testing.T) {
	// Create a minimal config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
server:
  host: "localhost"
  port: 8080

database:
  path: "./data/app.db"

logging:
  level: "info"
  format: "text"

vaults:
  - id: "default"
    name: "Default"
    storage:
      type: "local"
      local:
        path: "./vault"
    index_path: "./indexes/default"
    db_path: "./db/default.db"
    enabled: true
    default: true

search:
  default_limit: 20
  max_limit: 100

indexing:
  batch_size: 100
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Set environment variables
	os.Setenv("OBSIDIAN_WEB_SERVER_HOST", "0.0.0.0")
	os.Setenv("OBSIDIAN_WEB_SERVER_PORT", "9999")
	os.Setenv("OBSIDIAN_WEB_DATABASE_PATH", "/custom/db.db")
	os.Setenv("OBSIDIAN_WEB_LOG_LEVEL", "debug")
	os.Setenv("OBSIDIAN_WEB_LOG_FORMAT", "json")
	os.Setenv("OBSIDIAN_WEB_VAULT_PATH", "/custom/vault")
	os.Setenv("OBSIDIAN_WEB_INDEX_PATH", "/custom/index")
	os.Setenv("OBSIDIAN_WEB_VAULT_DB_PATH", "/custom/db")
	os.Setenv("OBSIDIAN_WEB_SEARCH_DEFAULT_LIMIT", "30")

	defer func() {
		os.Unsetenv("OBSIDIAN_WEB_SERVER_HOST")
		os.Unsetenv("OBSIDIAN_WEB_SERVER_PORT")
		os.Unsetenv("OBSIDIAN_WEB_DATABASE_PATH")
		os.Unsetenv("OBSIDIAN_WEB_LOG_LEVEL")
		os.Unsetenv("OBSIDIAN_WEB_LOG_FORMAT")
		os.Unsetenv("OBSIDIAN_WEB_VAULT_PATH")
		os.Unsetenv("OBSIDIAN_WEB_INDEX_PATH")
		os.Unsetenv("OBSIDIAN_WEB_VAULT_DB_PATH")
		os.Unsetenv("OBSIDIAN_WEB_SEARCH_DEFAULT_LIMIT")
	}()

	// Load config
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment overrides
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Env override failed: expected host '0.0.0.0', got '%s'", cfg.Server.Host)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("Env override failed: expected port 9999, got %d", cfg.Server.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Env override failed: expected log level 'debug', got '%s'", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Env override failed: expected log format 'json', got '%s'", cfg.Logging.Format)
	}
	localCfg := cfg.Vaults[0].Storage.GetLocalConfig()
	if localCfg == nil || localCfg.Path != "/custom/vault" {
		if localCfg == nil {
			t.Error("Env override failed: expected local storage config, got nil")
		} else {
			t.Errorf("Env override failed: expected vault path '/custom/vault', got '%s'", localCfg.Path)
		}
	}
	if cfg.Vaults[0].IndexPath != "/custom/index" {
		t.Errorf("Env override failed: expected index path '/custom/index', got '%s'", cfg.Vaults[0].IndexPath)
	}
	if cfg.Vaults[0].DBPath != "/custom/db" {
		t.Errorf("Env override failed: expected db path '/custom/db', got '%s'", cfg.Vaults[0].DBPath)
	}
	if cfg.Search.DefaultLimit != 30 {
		t.Errorf("Env override failed: expected default limit 30, got %d", cfg.Search.DefaultLimit)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid default config",
			config:    DefaultConfig(),
			wantError: false,
		},
		{
			name: "invalid port - too low",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 0},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp"}}, IndexPath: "/tmp/idx", DBPath: "/tmp/db", Default: true, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "port must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 99999},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp"}}, IndexPath: "/tmp/idx", DBPath: "/tmp/db", Default: true, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "port must be between 1 and 65535",
		},
		{
			name: "empty database path",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 8080},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp"}}, IndexPath: "/tmp/idx", DBPath: "", Default: true, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "db_path cannot be empty",
		},
		{
			name: "invalid log level",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 8080},
				Logging: LoggingConfig{Level: "invalid", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp"}}, IndexPath: "/tmp/idx", DBPath: "/tmp/db", Default: true, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "logging.level must be one of",
		},
		{
			name: "no vaults configured",
			config: &Config{
				Server:   ServerConfig{Host: "localhost", Port: 8080},
				Logging:  LoggingConfig{Level: "info", Format: "text"},
				Vaults:   []VaultConfig{},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "at least one vault must be configured",
		},
		{
			name: "no default vault",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 8080},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp"}}, IndexPath: "/tmp/idx", DBPath: "/tmp/db", Default: false, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "at least one vault must be marked as default",
		},
		{
			name: "duplicate vault IDs",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 8080},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test1", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp1"}}, IndexPath: "/tmp/idx1", DBPath: "/tmp/db1", Default: true, Enabled: true},
					{ID: "test", Name: "Test2", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp2"}}, IndexPath: "/tmp/idx2", DBPath: "/tmp/db2", Default: false, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "duplicate vault ID",
		},
		{
			name: "S3 storage missing bucket",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 8080},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test", Storage: StorageConfig{Type: "s3", S3: &S3StorageConfig{Region: "us-east-1"}}, IndexPath: "/tmp/idx", DBPath: "/tmp/db", Default: true, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "bucket cannot be empty",
		},
		{
			name: "S3 storage missing region",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 8080},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test", Storage: StorageConfig{Type: "s3", S3: &S3StorageConfig{Bucket: "my-bucket"}}, IndexPath: "/tmp/idx", DBPath: "/tmp/db", Default: true, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "region cannot be empty",
		},
		{
			name: "MinIO storage missing endpoint",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 8080},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test", Storage: StorageConfig{Type: "minio", MinIO: &MinIOStorageConfig{Bucket: "my-bucket"}}, IndexPath: "/tmp/idx", DBPath: "/tmp/db", Default: true, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "endpoint cannot be empty",
		},
		{
			name: "invalid storage type",
			config: &Config{
				Server:  ServerConfig{Host: "localhost", Port: 8080},
				Logging: LoggingConfig{Level: "info", Format: "text"},
				Vaults: []VaultConfig{
					{ID: "test", Name: "Test", Storage: StorageConfig{Type: "invalid"}, IndexPath: "/tmp/idx", DBPath: "/tmp/db", Default: true, Enabled: true},
				},
				Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
				Indexing: IndexingConfig{BatchSize: 100},
			},
			wantError: true,
			errorMsg:  "storage.type must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantError && err == nil {
				t.Errorf("Expected validation error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
			if tt.wantError && err != nil && tt.errorMsg != "" {
				if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errorMsg, err)
				}
			}
		})
	}
}

func TestGetVaultByID(t *testing.T) {
	cfg := &Config{
		Server:  ServerConfig{Host: "localhost", Port: 8080},
		Logging: LoggingConfig{Level: "info", Format: "text"},
		Vaults: []VaultConfig{
			{ID: "vault1", Name: "Vault 1", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp/v1"}}, IndexPath: "/tmp/idx1", DBPath: "/tmp/db1", Default: true, Enabled: true},
			{ID: "vault2", Name: "Vault 2", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp/v2"}}, IndexPath: "/tmp/idx2", DBPath: "/tmp/db2", Default: false, Enabled: true},
		},
		Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
		Indexing: IndexingConfig{BatchSize: 100},
	}

	// Test existing vault
	vault, err := cfg.GetVaultByID("vault1")
	if err != nil {
		t.Errorf("Expected to find vault1, got error: %v", err)
	}
	if vault.Name != "Vault 1" {
		t.Errorf("Expected vault name 'Vault 1', got '%s'", vault.Name)
	}

	// Test non-existing vault
	_, err = cfg.GetVaultByID("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent vault, got nil")
	}
}

func TestGetDefaultVault(t *testing.T) {
	cfg := &Config{
		Server:  ServerConfig{Host: "localhost", Port: 8080},
		Logging: LoggingConfig{Level: "info", Format: "text"},
		Vaults: []VaultConfig{
			{ID: "vault1", Name: "Vault 1", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp/v1"}}, IndexPath: "/tmp/idx1", DBPath: "/tmp/db1", Default: false, Enabled: true},
			{ID: "vault2", Name: "Vault 2", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp/v2"}}, IndexPath: "/tmp/idx2", DBPath: "/tmp/db2", Default: true, Enabled: true},
		},
		Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
		Indexing: IndexingConfig{BatchSize: 100},
	}

	vault := cfg.GetDefaultVault()
	if vault == nil {
		t.Fatal("Expected to find default vault, got nil")
	}
	if vault.ID != "vault2" {
		t.Errorf("Expected default vault ID 'vault2', got '%s'", vault.ID)
	}
}

func TestListEnabledVaults(t *testing.T) {
	cfg := &Config{
		Server:  ServerConfig{Host: "localhost", Port: 8080},
		Logging: LoggingConfig{Level: "info", Format: "text"},
		Vaults: []VaultConfig{
			{ID: "vault1", Name: "Vault 1", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp/v1"}}, IndexPath: "/tmp/idx1", DBPath: "/tmp/db1", Default: true, Enabled: true},
			{ID: "vault2", Name: "Vault 2", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp/v2"}}, IndexPath: "/tmp/idx2", DBPath: "/tmp/db2", Default: false, Enabled: false},
			{ID: "vault3", Name: "Vault 3", Storage: StorageConfig{Type: "local", Local: &LocalStorageConfig{Path: "/tmp/v3"}}, IndexPath: "/tmp/idx3", DBPath: "/tmp/db3", Default: false, Enabled: true},
		},
		Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
		Indexing: IndexingConfig{BatchSize: 100},
	}

	enabled := cfg.ListEnabledVaults()
	if len(enabled) != 2 {
		t.Errorf("Expected 2 enabled vaults, got %d", len(enabled))
	}
	if enabled[0].ID != "vault1" || enabled[1].ID != "vault3" {
		t.Error("Enabled vaults list is incorrect")
	}
}

func TestEnsureDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Server:  ServerConfig{Host: "localhost", Port: 8080},
		Logging: LoggingConfig{Level: "info", Format: "text"},
		Vaults: []VaultConfig{
			{
				ID:   "test",
				Name: "Test",
				Storage: StorageConfig{
					Type: "local",
					Local: &LocalStorageConfig{
						Path: filepath.Join(tmpDir, "vault"),
					},
				},
				IndexPath: filepath.Join(tmpDir, "indexes", "test"),
				DBPath:    filepath.Join(tmpDir, "data", "test.db"),
				Default:   true,
				Enabled:   true,
			},
		},
		Search:   SearchConfig{DefaultLimit: 20, MaxLimit: 100},
		Indexing: IndexingConfig{BatchSize: 100},
	}

	if err := cfg.EnsureDirectories(); err != nil {
		t.Fatalf("EnsureDirectories failed: %v", err)
	}

	// Check if directories were created
	if _, err := os.Stat(filepath.Join(tmpDir, "data")); os.IsNotExist(err) {
		t.Error("Database directory was not created")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "vault")); os.IsNotExist(err) {
		t.Error("Vault directory was not created")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "indexes", "test")); os.IsNotExist(err) {
		t.Error("Index directory was not created")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "data", "test.db")); os.IsNotExist(err) {
		t.Error("Index directory was not created")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
