package vault

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/indexing"
	"github.com/susamn/obsidian-web/internal/search"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// VaultStatus represents the current state of a vault
type VaultStatus int

const (
	VaultStatusInitializing VaultStatus = iota
	VaultStatusActive
	VaultStatusStopped
	VaultStatusError
)

func (s VaultStatus) String() string {
	switch s {
	case VaultStatusInitializing:
		return "initializing"
	case VaultStatusActive:
		return "active"
	case VaultStatusStopped:
		return "stopped"
	case VaultStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// FileOperation represents a file indexing operation
type FileOperation struct {
	Path      string
	Operation string // "create", "modify", "delete"
	Timestamp time.Time
}

// Vault represents a single Obsidian vault with its services
type Vault struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	config    *config.VaultConfig
	vaultPath string

	// Services
	syncService   *syncpkg.SyncService
	indexService  *indexing.IndexService
	searchService *search.SearchService

	// State
	status       VaultStatus
	startTime    time.Time
	stopChan     chan struct{}
	eventRouter  *sync.WaitGroup
	recentOps    []FileOperation // Last 10 operations
	maxRecentOps int
}

// VaultMetrics provides vault status and metrics
type VaultMetrics struct {
	VaultID          string
	VaultName        string
	Status           VaultStatus
	Uptime           time.Duration
	IndexedFiles     uint64
	RecentOperations []FileOperation
}

// NewVault creates a new vault instance with all services
func NewVault(ctx context.Context, cfg *config.VaultConfig) (*Vault, error) {
	if cfg == nil {
		return nil, fmt.Errorf("vault config cannot be nil")
	}

	if !cfg.Enabled {
		return nil, fmt.Errorf("vault %s is disabled", cfg.ID)
	}

	// Determine vault path based on storage type
	vaultPath, err := getVaultPath(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to determine vault path: %w", err)
	}

	// Create cancellable context
	vaultCtx, cancel := context.WithCancel(ctx)

	vault := &Vault{
		ctx:          vaultCtx,
		cancel:       cancel,
		config:       cfg,
		vaultPath:    vaultPath,
		status:       VaultStatusInitializing,
		stopChan:     make(chan struct{}),
		eventRouter:  &sync.WaitGroup{},
		recentOps:    make([]FileOperation, 0, 10),
		maxRecentOps: 10,
	}

	// Create services
	if err := vault.initializeServices(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}

	log.Printf("[%s] Vault initialized (path: %s, storage: %s)",
		cfg.ID, vaultPath, cfg.Storage.GetType())

	return vault, nil
}

// initializeServices creates all vault services
func (v *Vault) initializeServices() error {
	var err error

	// Create sync service
	v.syncService, err = syncpkg.NewSyncService(v.ctx, v.config.ID, &v.config.Storage)
	if err != nil {
		return fmt.Errorf("failed to create sync service: %w", err)
	}

	// Create index service
	v.indexService, err = indexing.NewIndexService(v.ctx, v.config, v.vaultPath)
	if err != nil {
		return fmt.Errorf("failed to create index service: %w", err)
	}

	// Create search service (it will be started after index is ready)
	v.searchService = search.NewSearchService(v.ctx, v.config.ID, v.indexService.GetIndex())

	// Register search service to receive index update notifications
	v.indexService.RegisterIndexNotifier(v.searchService)

	return nil
}

// Start starts all vault services
func (v *Vault) Start() error {
	v.mu.Lock()
	if v.status != VaultStatusInitializing && v.status != VaultStatusStopped {
		v.mu.Unlock()
		return fmt.Errorf("vault cannot start from state %s", v.status)
	}
	v.startTime = time.Now()
	v.mu.Unlock()

	// Start index service
	if err := v.indexService.Start(); err != nil {
		v.setStatus(VaultStatusError)
		return fmt.Errorf("failed to start index service: %w", err)
	}

	// Monitor index and start search
	go v.monitorIndexAndStartSearch()

	// Start sync service
	if err := v.syncService.Start(); err != nil {
		v.setStatus(VaultStatusError)
		return fmt.Errorf("failed to start sync service: %w", err)
	}

	// Start event router
	v.startEventRouter()

	return nil
}

// Resume resumes a stopped vault
func (v *Vault) Resume() error {
	v.mu.RLock()
	status := v.status
	v.mu.RUnlock()

	if status != VaultStatusStopped {
		return fmt.Errorf("cannot resume from state %s", status)
	}

	// Recreate context
	v.mu.Lock()
	vaultCtx, cancel := context.WithCancel(context.Background())
	v.ctx = vaultCtx
	v.cancel = cancel
	v.stopChan = make(chan struct{})
	v.mu.Unlock()

	return v.Start()
}

// monitorIndexAndStartSearch waits for index ready, then starts search
func (v *Vault) monitorIndexAndStartSearch() {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-v.ctx.Done():
			return
		case <-timeout:
			v.setStatus(VaultStatusError)
			return
		case <-ticker.C:
			status := v.indexService.GetStatus()
			if status == indexing.StatusReady {
				if err := v.searchService.Start(); err != nil {
					v.setStatus(VaultStatusError)
					return
				}
				v.setStatus(VaultStatusActive)
				return
			} else if status == indexing.StatusError {
				v.setStatus(VaultStatusError)
				return
			}
		}
	}
}

// startEventRouter connects sync events to index
func (v *Vault) startEventRouter() {
	v.eventRouter.Add(1)
	go func() {
		defer v.eventRouter.Done()
		for {
			select {
			case <-v.ctx.Done():
				return
			case <-v.stopChan:
				return
			case event, ok := <-v.syncService.Events():
				if !ok {
					return
				}
				v.trackFileOperation(event)
				v.indexService.UpdateIndex(event)
			}
		}
	}()
}

// Stop stops all vault services
func (v *Vault) Stop() error {
	v.mu.Lock()
	if v.status == VaultStatusStopped {
		v.mu.Unlock()
		return nil
	}
	v.mu.Unlock()

	close(v.stopChan)
	v.cancel()

	if v.syncService != nil {
		v.syncService.Stop()
	}

	v.eventRouter.Wait()

	if v.searchService != nil {
		v.searchService.Stop()
	}

	if v.indexService != nil {
		v.indexService.Stop()
	}

	v.setStatus(VaultStatusStopped)
	return nil
}

// GetMetrics returns vault status and metrics
func (v *Vault) GetMetrics() VaultMetrics {
	v.mu.RLock()
	defer v.mu.RUnlock()

	metrics := VaultMetrics{
		VaultID:   v.config.ID,
		VaultName: v.config.Name,
		Status:    v.status,
	}

	if !v.startTime.IsZero() {
		metrics.Uptime = time.Since(v.startTime)
	}

	if v.indexService != nil {
		if index := v.indexService.GetIndex(); index != nil {
			count, _ := index.DocCount()
			metrics.IndexedFiles = count
		}
	}

	ops := make([]FileOperation, len(v.recentOps))
	copy(ops, v.recentOps)
	metrics.RecentOperations = ops

	return metrics
}

// GetSyncService returns the sync service
func (v *Vault) GetSyncService() *syncpkg.SyncService {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.syncService
}

// GetIndexService returns the index service
func (v *Vault) GetIndexService() *indexing.IndexService {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.indexService
}

// GetSearchService returns the search service
func (v *Vault) GetSearchService() *search.SearchService {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.searchService
}

// GetIndex returns the underlying Bleve index
func (v *Vault) GetIndex() bleve.Index {
	if v.indexService != nil {
		return v.indexService.GetIndex()
	}
	return nil
}

// trackFileOperation adds a file operation to the history
func (v *Vault) trackFileOperation(event syncpkg.FileChangeEvent) {
	op := "modify"
	switch event.EventType {
	case syncpkg.FileCreated:
		op = "create"
	case syncpkg.FileModified:
		op = "modify"
	case syncpkg.FileDeleted:
		op = "delete"
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	fileOp := FileOperation{
		Path:      event.Path,
		Operation: op,
		Timestamp: event.Timestamp,
	}

	v.recentOps = append([]FileOperation{fileOp}, v.recentOps...)
	if len(v.recentOps) > v.maxRecentOps {
		v.recentOps = v.recentOps[:v.maxRecentOps]
	}
}

// GetStatus returns the current vault status
func (v *Vault) GetStatus() VaultStatus {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.status
}

// VaultID returns the vault ID
func (v *Vault) VaultID() string {
	return v.config.ID
}

// VaultName returns the vault name
func (v *Vault) VaultName() string {
	return v.config.Name
}

// IsActive returns true if vault is active and ready for operations
func (v *Vault) IsActive() bool {
	return v.GetStatus() == VaultStatusActive
}

// IsReady is an alias for IsActive for backwards compatibility
func (v *Vault) IsReady() bool {
	return v.IsActive()
}

// WaitForReady blocks until vault is active or timeout
func (v *Vault) WaitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for vault")
		}
		status := v.GetStatus()
		if status == VaultStatusActive {
			return nil
		}
		if status == VaultStatusError {
			return fmt.Errorf("vault error")
		}
		if status == VaultStatusStopped {
			return fmt.Errorf("vault stopped")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// setStatus sets the vault status
func (v *Vault) setStatus(status VaultStatus) {
	v.mu.Lock()
	v.status = status
	v.mu.Unlock()
}

// getVaultPath determines the local path for the vault based on storage type
func getVaultPath(cfg *config.VaultConfig) (string, error) {
	switch cfg.Storage.GetType() {
	case config.LocalStorage:
		localCfg := cfg.Storage.GetLocalConfig()
		if localCfg == nil || localCfg.Path == "" {
			return "", fmt.Errorf("local storage config missing or path empty")
		}
		return localCfg.Path, nil

	case config.S3Storage:
		// For S3, sync service will download to local cache
		// For now, return a placeholder - sync service handles this
		return fmt.Sprintf("/tmp/vault-cache/%s", cfg.ID), fmt.Errorf("S3 storage not yet implemented")

	case config.MinIOStorage:
		// For MinIO, sync service will download to local cache
		return fmt.Sprintf("/tmp/vault-cache/%s", cfg.ID), fmt.Errorf("MinIO storage not yet implemented")

	default:
		return "", fmt.Errorf("unknown storage type: %s", cfg.Storage.GetType())
	}
}
