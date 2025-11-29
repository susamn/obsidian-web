package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/susamn/obsidian-web/internal/config"
)

// FileEventType represents the type of file system event
type FileEventType int

const (
	FileCreated FileEventType = iota
	FileModified
	FileDeleted
)

// String returns the string representation of FileEventType
func (f FileEventType) String() string {
	switch f {
	case FileCreated:
		return "created"
	case FileModified:
		return "modified"
	case FileDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// FileChangeEvent represents a file change event
type FileChangeEvent struct {
	VaultID   string
	Path      string
	EventType FileEventType
	Timestamp time.Time
}

// syncBackend is internal interface for different sync implementations
type syncBackend interface {
	Start(ctx context.Context, events chan<- FileChangeEvent) error
	Stop() error
	ReIndex(events chan<- FileChangeEvent) error
}

// SyncService monitors storage backend for file changes
// All operations are non-blocking and run in goroutines
type SyncService struct {
	ctx      context.Context
	cancel   context.CancelFunc
	vaultID  string
	storage  *config.StorageConfig
	events   chan FileChangeEvent
	backend  syncBackend
	wg       sync.WaitGroup
	startErr error
	mu       sync.RWMutex
}

// NewSyncService creates a new sync service for a vault
// The service starts monitoring in a non-blocking goroutine
func NewSyncService(ctx context.Context, vaultID string, storage *config.StorageConfig) (*SyncService, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage config cannot be nil")
	}

	// Create a cancellable context
	svcCtx, cancel := context.WithCancel(ctx)

	// Create buffered channel to prevent blocking
	// Large buffer (10000) to handle bulk operations without dropping events
	events := make(chan FileChangeEvent, 10000)

	// Create the appropriate backend based on storage type
	var backend syncBackend
	var err error

	storageType := storage.GetType()
	switch storageType {
	case config.LocalStorage:
		localCfg := storage.GetLocalConfig()
		if localCfg == nil {
			cancel()
			return nil, fmt.Errorf("local storage config is nil")
		}
		backend, err = newLocalSync(vaultID, localCfg.Path)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create local sync: %w", err)
		}

	case config.S3Storage:
		s3Cfg := storage.GetS3Config()
		if s3Cfg == nil {
			cancel()
			return nil, fmt.Errorf("s3 storage config is nil")
		}
		backend = newS3Sync(vaultID, s3Cfg)

	case config.MinIOStorage:
		minioCfg := storage.GetMinIOConfig()
		if minioCfg == nil {
			cancel()
			return nil, fmt.Errorf("minio storage config is nil")
		}
		backend = newMinIOSync(vaultID, minioCfg)

	default:
		cancel()
		return nil, fmt.Errorf("unsupported storage type: %s", storageType)
	}

	service := &SyncService{
		ctx:     svcCtx,
		cancel:  cancel,
		vaultID: vaultID,
		storage: storage,
		events:  events,
		backend: backend,
	}

	return service, nil
}

// Start begins monitoring the storage backend in a non-blocking goroutine
func (s *SyncService) Start() error {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(s.events)

		// Start the backend (non-blocking)
		err := s.backend.Start(s.ctx, s.events)
		if err != nil {
			s.mu.Lock()
			s.startErr = err
			s.mu.Unlock()
		}
	}()

	return nil
}

// Events returns the read-only channel for file change events
func (s *SyncService) Events() <-chan FileChangeEvent {
	return s.events
}

// Stop stops the sync service gracefully
// Waits for all goroutines to finish
func (s *SyncService) Stop() error {
	// Cancel the context to signal shutdown
	s.cancel()

	// Stop the backend
	if err := s.backend.Stop(); err != nil {
		return fmt.Errorf("failed to stop backend: %w", err)
	}

	// Wait for all goroutines to finish
	s.wg.Wait()

	// Check if there was a start error
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.startErr != nil {
		return fmt.Errorf("sync service error: %w", s.startErr)
	}

	return nil
}

// VaultID returns the vault ID this service is monitoring
func (s *SyncService) VaultID() string {
	return s.vaultID
}

// PendingEventsCount returns the number of pending events in the channel
// This is a non-blocking operation that returns the current buffer length
func (s *SyncService) PendingEventsCount() int {
	return len(s.events)
}

// InjectEvent injects an event back into the sync channel (for retries)
// Returns true if event was injected, false if channel is full
// Non-blocking operation
func (s *SyncService) InjectEvent(event FileChangeEvent) bool {
	select {
	case s.events <- event:
		return true
	default:
		return false
	}
}

// ReIndex triggers a full re-index of the vault
// It walks the entire filesystem and emits FileCreated events for all files
// This operation runs asynchronously on the backend but this method blocks until the walk is started
func (s *SyncService) ReIndex() error {
	return s.backend.ReIndex(s.events)
}
