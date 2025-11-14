package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Logging  LoggingConfig  `yaml:"logging"`
	Vaults   []VaultConfig  `yaml:"vaults"`
	Search   SearchConfig   `yaml:"search"`
	Indexing IndexingConfig `yaml:"indexing"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// DatabaseConfig holds SQLite database configuration
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
	Output string `yaml:"output"` // stdout, stderr, or file path
}

// VaultConfig represents a single vault configuration
type VaultConfig struct {
	ID        string        `yaml:"id"`
	Name      string        `yaml:"name"`
	Storage   StorageConfig `yaml:"storage"`
	IndexPath string        `yaml:"index_path"`
	Enabled   bool          `yaml:"enabled"`
	Default   bool          `yaml:"default"`
}

// StorageType represents the type of storage backend
type StorageType int

const (
	LocalStorage StorageType = iota
	S3Storage
	MinIOStorage
)

// String returns the string representation of StorageType
func (s StorageType) String() string {
	switch s {
	case LocalStorage:
		return "local"
	case S3Storage:
		return "s3"
	case MinIOStorage:
		return "minio"
	default:
		return "unknown"
	}
}

// LocalStorageConfig holds local filesystem storage configuration
type LocalStorageConfig struct {
	Path string `yaml:"path"`
}

// S3StorageConfig holds S3 storage configuration
type S3StorageConfig struct {
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	Endpoint  string `yaml:"endpoint,omitempty"`   // For S3-compatible services
	AccessKey string `yaml:"access_key,omitempty"` // Optional, can use env vars
	SecretKey string `yaml:"secret_key,omitempty"` // Optional, can use env vars
}

// MinIOStorageConfig holds MinIO storage configuration
type MinIOStorageConfig struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key,omitempty"`
	SecretKey string `yaml:"secret_key,omitempty"`
	UseSSL    bool   `yaml:"use_ssl"`
}

// StorageConfig holds storage backend configuration
// Only one of Local, S3, or MinIO should be set based on Type
type StorageConfig struct {
	Type  string              `yaml:"type"` // "local", "s3", "minio"
	Local *LocalStorageConfig `yaml:"local,omitempty"`
	S3    *S3StorageConfig    `yaml:"s3,omitempty"`
	MinIO *MinIOStorageConfig `yaml:"minio,omitempty"`
}

// GetType returns the StorageType enum value
func (s *StorageConfig) GetType() StorageType {
	switch s.Type {
	case "local":
		return LocalStorage
	case "s3":
		return S3Storage
	case "minio":
		return MinIOStorage
	default:
		return -1
	}
}

// GetConfig returns the underlying storage configuration as interface{}
func (s *StorageConfig) GetConfig() interface{} {
	switch s.GetType() {
	case LocalStorage:
		return s.Local
	case S3Storage:
		return s.S3
	case MinIOStorage:
		return s.MinIO
	default:
		return nil
	}
}

// GetLocalConfig returns LocalStorageConfig if type is local
func (s *StorageConfig) GetLocalConfig() *LocalStorageConfig {
	if s.GetType() == LocalStorage {
		return s.Local
	}
	return nil
}

// GetS3Config returns S3StorageConfig if type is s3
func (s *StorageConfig) GetS3Config() *S3StorageConfig {
	if s.GetType() == S3Storage {
		return s.S3
	}
	return nil
}

// GetMinIOConfig returns MinIOStorageConfig if type is minio
func (s *StorageConfig) GetMinIOConfig() *MinIOStorageConfig {
	if s.GetType() == MinIOStorage {
		return s.MinIO
	}
	return nil
}

// SearchConfig holds search-related configuration
type SearchConfig struct {
	DefaultLimit int `yaml:"default_limit"`
	MaxLimit     int `yaml:"max_limit"`
	CacheSizeMB  int `yaml:"cache_size_mb"`
}

// IndexingConfig holds indexing-related configuration
type IndexingConfig struct {
	BatchSize          int           `yaml:"batch_size"`
	AutoIndexOnStartup bool          `yaml:"auto_index_on_startup"`
	WatchForChanges    bool          `yaml:"watch_for_changes"`
	UpdateInterval     time.Duration `yaml:"update_interval"`
}

// LoadConfig loads configuration with fallback priority:
// 1. Provided configPath parameter
// 2. OBSIDIAN_WEB_CONFIG_PATH environment variable
// 3. ~/.config/obsidian-web/config.yaml
// 4. ./config.yaml
// 5. /etc/obsidian-web/config.yaml
// 6. Built-in defaults
func LoadConfig(configPath string) (*Config, error) {
	var cfg *Config
	var err error
	var loadedFrom string

	// 1. Use provided path if specified
	if configPath != "" {
		cfg, err = loadFromFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
		}
		loadedFrom = configPath
	} else {
		// 2. Check environment variable
		if envPath := os.Getenv("OBSIDIAN_WEB_CONFIG_PATH"); envPath != "" {
			cfg, err = loadFromFile(envPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load config from env path %s: %w", envPath, err)
			}
			loadedFrom = envPath
		} else {
			// 3. Check standard locations
			homeDir, _ := os.UserHomeDir()
			searchPaths := []string{
				filepath.Join(homeDir, ".config", "obsidian-web", "config.yaml"),
				"./config.yaml",
				"/etc/obsidian-web/config.yaml",
			}

			for _, path := range searchPaths {
				if _, err := os.Stat(path); err == nil {
					cfg, err = loadFromFile(path)
					if err != nil {
						return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
					}
					loadedFrom = path
					break
				}
			}

			// 4. Use defaults if no config file found
			if cfg == nil {
				cfg = DefaultConfig()
				loadedFrom = "built-in defaults"
			}
		}
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	// Apply default config directory paths if using home directory
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		configDir := filepath.Join(homeDir, ".config", "obsidian-web")

		// If database path is default, use home config directory
		if cfg.Database.Path == "./data/app.db" || cfg.Database.Path == "" {
			cfg.Database.Path = filepath.Join(configDir, "app.db")
		}

		// Update vault paths if they are relative defaults
		for i := range cfg.Vaults {
			if cfg.Vaults[i].Storage.GetType() == LocalStorage {
				localCfg := cfg.Vaults[i].Storage.GetLocalConfig()
				if localCfg != nil && (localCfg.Path == "./data/vault" || localCfg.Path == "") {
					localCfg.Path = filepath.Join(configDir, "vault")
				}
			}

			if cfg.Vaults[i].IndexPath == "" || cfg.Vaults[i].IndexPath == "./data/indexes/default" {
				cfg.Vaults[i].IndexPath = filepath.Join(configDir, "indexes", cfg.Vaults[i].ID)
			}
		}
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config (loaded from %s): %w", loadedFrom, err)
	}

	return cfg, nil
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Unmarshal YAML into config
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return cfg, nil
}

// applyEnvOverrides allows environment variables to override config file values
func (c *Config) applyEnvOverrides() {
	// Server overrides
	if host := os.Getenv("OBSIDIAN_WEB_SERVER_HOST"); host != "" {
		c.Server.Host = host
	}
	if port := os.Getenv("OBSIDIAN_WEB_SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Server.Port = p
		}
	}
	if timeout := os.Getenv("OBSIDIAN_WEB_SERVER_READ_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			c.Server.ReadTimeout = d
		}
	}
	if timeout := os.Getenv("OBSIDIAN_WEB_SERVER_WRITE_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			c.Server.WriteTimeout = d
		}
	}

	// Database overrides
	if dbPath := os.Getenv("OBSIDIAN_WEB_DATABASE_PATH"); dbPath != "" {
		c.Database.Path = dbPath
	}

	// Logging overrides
	if level := os.Getenv("OBSIDIAN_WEB_LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}
	if format := os.Getenv("OBSIDIAN_WEB_LOG_FORMAT"); format != "" {
		c.Logging.Format = format
	}
	if output := os.Getenv("OBSIDIAN_WEB_LOG_OUTPUT"); output != "" {
		c.Logging.Output = output
	}

	// Search overrides
	if limit := os.Getenv("OBSIDIAN_WEB_SEARCH_DEFAULT_LIMIT"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			c.Search.DefaultLimit = l
		}
	}
	if limit := os.Getenv("OBSIDIAN_WEB_SEARCH_MAX_LIMIT"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			c.Search.MaxLimit = l
		}
	}

	// Indexing overrides
	if batchSize := os.Getenv("OBSIDIAN_WEB_INDEXING_BATCH_SIZE"); batchSize != "" {
		if bs, err := strconv.Atoi(batchSize); err == nil {
			c.Indexing.BatchSize = bs
		}
	}
	if autoIndex := os.Getenv("OBSIDIAN_WEB_INDEXING_AUTO_INDEX"); autoIndex != "" {
		c.Indexing.AutoIndexOnStartup = autoIndex == "true" || autoIndex == "1"
	}

	// Simple vault override (for single-vault deployments)
	if vaultPath := os.Getenv("OBSIDIAN_WEB_VAULT_PATH"); vaultPath != "" {
		if len(c.Vaults) > 0 && c.Vaults[0].Storage.GetType() == LocalStorage {
			localCfg := c.Vaults[0].Storage.GetLocalConfig()
			if localCfg != nil {
				localCfg.Path = vaultPath
			}
		}
	}
	if indexPath := os.Getenv("OBSIDIAN_WEB_INDEX_PATH"); indexPath != "" {
		if len(c.Vaults) > 0 {
			c.Vaults[0].IndexPath = indexPath
		}
	}

	// S3/MinIO credentials from standard AWS env vars
	if accessKey := os.Getenv("AWS_ACCESS_KEY_ID"); accessKey != "" {
		for i := range c.Vaults {
			storageType := c.Vaults[i].Storage.GetType()
			if storageType == S3Storage {
				s3Cfg := c.Vaults[i].Storage.GetS3Config()
				if s3Cfg != nil {
					s3Cfg.AccessKey = accessKey
				}
			} else if storageType == MinIOStorage {
				minioCfg := c.Vaults[i].Storage.GetMinIOConfig()
				if minioCfg != nil {
					minioCfg.AccessKey = accessKey
				}
			}
		}
	}
	if secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY"); secretKey != "" {
		for i := range c.Vaults {
			storageType := c.Vaults[i].Storage.GetType()
			if storageType == S3Storage {
				s3Cfg := c.Vaults[i].Storage.GetS3Config()
				if s3Cfg != nil {
					s3Cfg.SecretKey = secretKey
				}
			} else if storageType == MinIOStorage {
				minioCfg := c.Vaults[i].Storage.GetMinIOConfig()
				if minioCfg != nil {
					minioCfg.SecretKey = secretKey
				}
			}
		}
	}
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "localhost",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Path: "./data/app.db",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
		Vaults: []VaultConfig{
			{
				ID:   "default",
				Name: "Default Vault",
				Storage: StorageConfig{
					Type: "local",
					Local: &LocalStorageConfig{
						Path: "./data/vault",
					},
				},
				IndexPath: "./data/indexes/default",
				Enabled:   true,
				Default:   true,
			},
		},
		Search: SearchConfig{
			DefaultLimit: 20,
			MaxLimit:     100,
			CacheSizeMB:  512,
		},
		Indexing: IndexingConfig{
			BatchSize:          100,
			AutoIndexOnStartup: true,
			WatchForChanges:    false,
			UpdateInterval:     5 * time.Minute,
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Server validation
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Server.Host == "" {
		return fmt.Errorf("server.host cannot be empty")
	}
	if c.Server.ReadTimeout < 0 {
		return fmt.Errorf("server.read_timeout cannot be negative")
	}
	if c.Server.WriteTimeout < 0 {
		return fmt.Errorf("server.write_timeout cannot be negative")
	}

	// Database validation
	if c.Database.Path == "" {
		return fmt.Errorf("database.path cannot be empty")
	}

	// Logging validation
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error; got %s", c.Logging.Level)
	}
	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[c.Logging.Format] {
		return fmt.Errorf("logging.format must be one of: json, text; got %s", c.Logging.Format)
	}

	// Vault validation
	if len(c.Vaults) == 0 {
		return fmt.Errorf("at least one vault must be configured")
	}

	defaultCount := 0
	vaultIDs := make(map[string]bool)
	for i, vault := range c.Vaults {
		if vault.ID == "" {
			return fmt.Errorf("vaults[%d].id cannot be empty", i)
		}
		if vaultIDs[vault.ID] {
			return fmt.Errorf("duplicate vault ID: %s", vault.ID)
		}
		vaultIDs[vault.ID] = true

		if vault.Name == "" {
			return fmt.Errorf("vaults[%d].name cannot be empty", i)
		}
		if vault.IndexPath == "" {
			return fmt.Errorf("vaults[%d].index_path cannot be empty", i)
		}

		// Storage validation
		validStorageTypes := map[string]bool{"local": true, "s3": true, "minio": true}
		if !validStorageTypes[vault.Storage.Type] {
			return fmt.Errorf("vaults[%d].storage.type must be one of: local, s3, minio; got %s", i, vault.Storage.Type)
		}

		storageType := vault.Storage.GetType()
		switch storageType {
		case LocalStorage:
			localCfg := vault.Storage.GetLocalConfig()
			if localCfg == nil {
				return fmt.Errorf("vaults[%d].storage.local configuration is required for local storage", i)
			}
			if localCfg.Path == "" {
				return fmt.Errorf("vaults[%d].storage.local.path cannot be empty for local storage", i)
			}
		case S3Storage:
			s3Cfg := vault.Storage.GetS3Config()
			if s3Cfg == nil {
				return fmt.Errorf("vaults[%d].storage.s3 configuration is required for S3 storage", i)
			}
			if s3Cfg.Bucket == "" {
				return fmt.Errorf("vaults[%d].storage.s3.bucket cannot be empty for S3 storage", i)
			}
			if s3Cfg.Region == "" {
				return fmt.Errorf("vaults[%d].storage.s3.region cannot be empty for S3 storage", i)
			}
		case MinIOStorage:
			minioCfg := vault.Storage.GetMinIOConfig()
			if minioCfg == nil {
				return fmt.Errorf("vaults[%d].storage.minio configuration is required for MinIO storage", i)
			}
			if minioCfg.Bucket == "" {
				return fmt.Errorf("vaults[%d].storage.minio.bucket cannot be empty for MinIO storage", i)
			}
			if minioCfg.Endpoint == "" {
				return fmt.Errorf("vaults[%d].storage.minio.endpoint cannot be empty for MinIO storage", i)
			}
		default:
			return fmt.Errorf("vaults[%d].storage.type is invalid", i)
		}

		if vault.Default {
			defaultCount++
		}
	}

	if defaultCount == 0 {
		return fmt.Errorf("at least one vault must be marked as default")
	}
	if defaultCount > 1 {
		return fmt.Errorf("only one vault can be marked as default, found %d", defaultCount)
	}

	// Search validation
	if c.Search.DefaultLimit < 1 {
		return fmt.Errorf("search.default_limit must be at least 1, got %d", c.Search.DefaultLimit)
	}
	if c.Search.MaxLimit < c.Search.DefaultLimit {
		return fmt.Errorf("search.max_limit (%d) must be >= search.default_limit (%d)", c.Search.MaxLimit, c.Search.DefaultLimit)
	}

	// Indexing validation
	if c.Indexing.BatchSize < 1 {
		return fmt.Errorf("indexing.batch_size must be at least 1, got %d", c.Indexing.BatchSize)
	}

	return nil
}

// GetDefaultVault returns the default vault configuration
func (c *Config) GetDefaultVault() *VaultConfig {
	for i := range c.Vaults {
		if c.Vaults[i].Default {
			return &c.Vaults[i]
		}
	}
	return nil
}

// GetVaultByID returns a vault by its ID
func (c *Config) GetVaultByID(id string) (*VaultConfig, error) {
	for i := range c.Vaults {
		if c.Vaults[i].ID == id {
			return &c.Vaults[i], nil
		}
	}
	return nil, fmt.Errorf("vault not found: %s", id)
}

// ListEnabledVaults returns all enabled vaults
func (c *Config) ListEnabledVaults() []VaultConfig {
	var enabled []VaultConfig
	for _, vault := range c.Vaults {
		if vault.Enabled {
			enabled = append(enabled, vault)
		}
	}
	return enabled
}

// EnsureDirectories creates necessary directories if they don't exist
func (c *Config) EnsureDirectories() error {
	// Create database directory
	dbDir := filepath.Dir(c.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
	}

	// Create vault directories and index directories
	for _, vault := range c.Vaults {
		if !vault.Enabled {
			continue
		}

		// Create local storage directory
		if vault.Storage.GetType() == LocalStorage {
			localCfg := vault.Storage.GetLocalConfig()
			if localCfg != nil && localCfg.Path != "" {
				if err := os.MkdirAll(localCfg.Path, 0755); err != nil {
					return fmt.Errorf("failed to create vault directory %s: %w", localCfg.Path, err)
				}
			}
		}

		// Create index directory
		if err := os.MkdirAll(vault.IndexPath, 0755); err != nil {
			return fmt.Errorf("failed to create index directory %s: %w", vault.IndexPath, err)
		}
	}

	return nil
}
