package vault

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/susamn/obsidian-web/internal/config"
	"github.com/susamn/obsidian-web/internal/db"
	"github.com/susamn/obsidian-web/internal/explorer"
	"github.com/susamn/obsidian-web/internal/indexing"
	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/recon"
	"github.com/susamn/obsidian-web/internal/search"
	"github.com/susamn/obsidian-web/internal/sse"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// VaultStatus represents the current state of a vault
type VaultStatus int

const (
	VaultStatusInitializing VaultStatus = iota
	VaultStatusActive
	VaultStatusReindexing
	VaultStatusStopped
	VaultStatusError
)

func (s VaultStatus) String() string {
	switch s {
	case VaultStatusInitializing:
		return "initializing"
	case VaultStatusActive:
		return "active"
	case VaultStatusReindexing:
		return "reindexing"
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
	syncService     *syncpkg.SyncService
	indexService    *indexing.IndexService
	searchService   *search.SearchService
	explorerService *explorer.ExplorerService
	dbService       *db.DBService

	// Event processing
	reconService *recon.ReconciliationService
	workers      []*Worker
	sseManager   *sse.Manager

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

	logger.WithFields(map[string]interface{}{
		"vault_id": cfg.ID,
		"path":     vaultPath,
		"storage":  cfg.Storage.GetType(),
	}).Info("Vault initialized")

	return vault, nil
}

// initializeServices creates all vault services
func (v *Vault) initializeServices() error {
	var err error

	dbPath := fmt.Sprintf("%s/vault_%s.db", v.config.DBPath, v.config.ID)
	// Create and start db service
	v.dbService, err = db.NewDBService(v.ctx, &dbPath)
	if err != nil {
		return fmt.Errorf("failed to create db service: %w", err)
	}

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

	// Create explorer service
	v.explorerService, err = explorer.NewExplorerService(v.ctx, v.config.ID, v.vaultPath, v.dbService)
	if err != nil {
		return fmt.Errorf("failed to create explorer service: %w", err)
	}

	// Create workers - reconciliation service will be created after sync service
	const numWorkers = 2
	v.workers = make([]*Worker, numWorkers)

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

	if err := v.dbService.Start(); err != nil {
		return fmt.Errorf("failed to start db service: %w", err)
	}

	// Start index service
	if err := v.indexService.Start(); err != nil {
		v.setStatus(VaultStatusError)
		return fmt.Errorf("failed to start index service: %w", err)
	}

	// Monitor index and start search
	go v.monitorIndexAndStartSearch(v.ctx)

	// Start explorer service
	if err := v.explorerService.Start(); err != nil {
		v.setStatus(VaultStatusError)
		return fmt.Errorf("failed to start explorer service: %w", err)
	}

	// Start sync service
	if err := v.syncService.Start(); err != nil {
		v.setStatus(VaultStatusError)
		return fmt.Errorf("failed to start sync service: %w", err)
	}

	// Get sync events channel
	syncEvents := v.syncService.Events()

	// Create reconciliation service
	v.reconService = recon.NewReconciliationService(
		v.config.ID,
		v.ctx,
		v.eventRouter,
		v.dbService,
		v.explorerService,
		v.indexService,
		v.sseManager,
		func(reindexing bool) {
			if reindexing {
				v.setStatus(VaultStatusReindexing)
			} else {
				v.setStatus(VaultStatusActive)
			}
		},
	)

	// Set sync service reference for retrying events
	v.reconService.SetSyncService(v.syncService)

	// Start reconciliation service
	v.reconService.Start()

	// Create and start workers with reconciliation service
	const numWorkers = 2
	for i := 0; i < numWorkers; i++ {
		v.workers[i] = NewWorker(
			i,
			v.config.ID,
			v.vaultPath,
			v.ctx,
			v.eventRouter,
			v.dbService,
			v.indexService,
			v.explorerService,
			v.reconService,
		)
		v.workers[i].Start(syncEvents)
	}

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
func (v *Vault) monitorIndexAndStartSearch(ctx context.Context) {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	searchServiceStarted := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			v.setStatus(VaultStatusError)
			return
		case <-ticker.C:
			indexStatus := v.indexService.GetStatus()

			// Wait for index to be ready first
			if !searchServiceStarted {
				if indexStatus == indexing.StatusReady {
					if err := v.searchService.Start(); err != nil {
						v.setStatus(VaultStatusError)
						return
					}
					searchServiceStarted = true
				} else if indexStatus == indexing.StatusError {
					v.setStatus(VaultStatusError)
					return
				}
				continue
			}

			// Once search service is started, wait for it to be ready
			searchStatus := v.searchService.GetStatus()
			if searchStatus == search.StatusReady {
				v.setStatus(VaultStatusActive)
				return
			} else if searchStatus == search.StatusError {
				v.setStatus(VaultStatusError)
				return
			}
		}
	}
}

// SetSSEManager sets the SSE manager for the vault
func (v *Vault) SetSSEManager(manager *sse.Manager) {
	v.mu.Lock()
	v.sseManager = manager
	v.mu.Unlock()

	// Register pending count getter for this vault
	// Tracks sync channel + reconciliation DLQ
	manager.RegisterPendingCountGetter(v.config.ID, func() int {
		count := 0
		if v.syncService != nil {
			count = v.syncService.PendingEventsCount()
		}
		if v.reconService != nil {
			count += v.reconService.GetDLQDepth()
		}
		return count
	})

	// Update workers with SSE manager
	for _, worker := range v.workers {
		worker.sseManager = manager
	}
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

	// Stop sync service first (stops producing events and closes sync channel)
	if v.syncService != nil {
		v.syncService.Stop()
	}

	// Unregister pending count getter from SSE manager
	v.mu.RLock()
	if v.sseManager != nil {
		v.sseManager.UnregisterPendingCountGetter(v.config.ID)
	}
	v.mu.RUnlock()

	// Wait for all workers to finish (they'll exit when sync channel closes)
	v.eventRouter.Wait()

	if v.explorerService != nil {
		v.explorerService.Stop()
	}

	if v.searchService != nil {
		v.searchService.Stop()
	}

	if v.indexService != nil {
		v.indexService.Stop()
	}

	if v.dbService != nil {
		v.dbService.Stop()
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
		index := v.indexService.GetIndex()
		if index != nil {
			count, err := index.DocCount()
			if err == nil {
				metrics.IndexedFiles = count
			}
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

// GetExplorerService returns the explorer service
func (v *Vault) GetExplorerService() *explorer.ExplorerService {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.explorerService
}

// GetDBService returns the database service
func (v *Vault) GetDBService() *db.DBService {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.dbService
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

// updateDatabase syncs file changes to the database
// Returns the file ID and error
func (v *Vault) updateDatabase(event syncpkg.FileChangeEvent) (string, error) {
	if v.dbService == nil {
		return "", fmt.Errorf("db service not available")
	}

	return v.dbService.PerformDatabaseUpdate(v.vaultPath, event)
}

// TriggerReindex triggers a full reindex of the vault via the reconciliation service
// This will:
// 1. Set vault status to Reindexing
// 2. Disable all files in DB (UI shows empty)
// 3. Clear explorer cache
// 4. Clear search index
// 5. Trigger sync service to re-walk filesystem and emit events
// 6. Workers rebuild DB/index/explorer as events are processed
// 7. Wait for sync channel to drain
// 8. Restore vault status to Active
func (v *Vault) TriggerReindex() {
	v.mu.RLock()
	reconService := v.reconService
	v.mu.RUnlock()

	if reconService != nil {
		reconService.TriggerReindex()
	} else {
		logger.WithField("vault_id", v.config.ID).Error("Cannot trigger reindex: reconciliation service not initialized")
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
