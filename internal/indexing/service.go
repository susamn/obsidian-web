package indexing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/susamn/obsidian-web/internal/config"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// ServiceStatus represents the current state of the index service
type ServiceStatus int

const (
	StatusStandby ServiceStatus = iota
	StatusInitialIndexing
	StatusReady
	StatusStopped
	StatusCancelled
	StatusError
)

// String returns the string representation of ServiceStatus
func (s ServiceStatus) String() string {
	switch s {
	case StatusStandby:
		return "standby"
	case StatusInitialIndexing:
		return "initial_indexing"
	case StatusReady:
		return "ready"
	case StatusStopped:
		return "stopped"
	case StatusCancelled:
		return "cancelled"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// StatusUpdate provides information about the indexing progress
type StatusUpdate struct {
	Status         ServiceStatus
	TotalCount     int
	RemainingCount int
	IndexedCount   int
	Message        string
	Error          error
}

// IndexUpdateEvent represents a notification that the index has been updated
// This is a lightweight notification - the index reference is only included
// when the index itself is rebuilt (rare). For incremental updates (common),
// NewIndex is nil and subscribers continue using their existing reference.
type IndexUpdateEvent struct {
	Timestamp time.Time
	EventType string      // "incremental" or "rebuild"
	NewIndex  bleve.Index // Only set for "rebuild" events, nil for "incremental"
}

// IndexUpdateNotifier is an interface for notifying about index updates
type IndexUpdateNotifier interface {
	NotifyIndexUpdate(event IndexUpdateEvent)
}

// IndexService provides indexing functionality for a vault
// It only works with local file paths - the sync service handles
// storage abstraction and ensures files are available locally
type IndexService struct {
	ctx        context.Context
	cancel     context.CancelFunc
	vaultID    string
	vaultName  string
	vaultPath  string // Local path to vault (or local cache for S3/MinIO)
	indexPath  string
	index      bleve.Index
	status     ServiceStatus
	statusChan chan StatusUpdate
	eventChan  chan syncpkg.FileChangeEvent // Input channel for sync events

	// Backpressure handling
	eventBuffer     int                                // Configurable buffer size
	batchSize       int                                // Number of events to batch together
	flushInterval   time.Duration                      // Max time before flushing batch
	pendingEvents   map[string]syncpkg.FileChangeEvent // Coalesced events by path
	pendingMu       sync.Mutex                         // Protects pendingEvents
	droppedEvents   int64                              // Counter for dropped events
	processedEvents int64                              // Counter for processed events

	// Index update notifications
	indexNotifiers   []IndexUpdateNotifier
	indexNotifiersMu sync.RWMutex

	wg sync.WaitGroup
	mu sync.RWMutex
}

// NewIndexService creates a new index service for a vault
// vaultPath should be a local filesystem path (sync service handles storage abstraction)
func NewIndexService(ctx context.Context, vault *config.VaultConfig, vaultPath string) (*IndexService, error) {
	if vault == nil {
		return nil, fmt.Errorf("vault config cannot be nil")
	}

	if vault.IndexPath == "" {
		return nil, fmt.Errorf("index path cannot be empty")
	}

	if vaultPath == "" {
		return nil, fmt.Errorf("vault path cannot be empty")
	}

	// Create a cancellable context
	svcCtx, cancel := context.WithCancel(ctx)

	// Create buffered status channel
	statusChan := make(chan StatusUpdate, 10)

	// Configure backpressure settings
	// Buffer size: larger for high-volume scenarios
	eventBuffer := 1000
	batchSize := 50
	flushInterval := 500 * time.Millisecond

	// Create buffered event channel for sync events
	eventChan := make(chan syncpkg.FileChangeEvent, eventBuffer)

	return &IndexService{
		ctx:           svcCtx,
		cancel:        cancel,
		vaultID:       vault.ID,
		vaultName:     vault.Name,
		vaultPath:     vaultPath,
		indexPath:     vault.IndexPath,
		status:        StatusStandby,
		statusChan:    statusChan,
		eventChan:     eventChan,
		eventBuffer:   eventBuffer,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		pendingEvents: make(map[string]syncpkg.FileChangeEvent),
	}, nil
}

// Start begins the initial indexing process in a non-blocking goroutine
func (s *IndexService) Start() error {
	s.mu.Lock()
	if s.status != StatusStandby {
		s.mu.Unlock()
		return fmt.Errorf("service already started or not in standby state")
	}
	s.status = StatusInitialIndexing
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(s.statusChan)

		if err := s.performInitialIndexing(); err != nil {
			s.updateStatus(StatusUpdate{
				Status:  StatusError,
				Message: "Initial indexing failed",
				Error:   err,
			})
			s.mu.Lock()
			s.status = StatusError
			s.mu.Unlock()
			return
		}

		// Check if context was cancelled
		select {
		case <-s.ctx.Done():
			s.updateStatus(StatusUpdate{
				Status:  StatusCancelled,
				Message: "Indexing cancelled",
			})
			s.mu.Lock()
			s.status = StatusCancelled
			s.mu.Unlock()
		default:
			s.updateStatus(StatusUpdate{
				Status:  StatusReady,
				Message: "Initial indexing completed",
			})
			s.mu.Lock()
			s.status = StatusReady
			s.mu.Unlock()
		}
	}()

	// Start event processing goroutine
	s.wg.Add(1)
	go s.processEvents()

	return nil
}

// performInitialIndexing performs the actual indexing work
func (s *IndexService) performInitialIndexing() error {
	log.Printf("[%s] Indexing vault from: %s", s.vaultID, s.vaultPath)
	log.Printf("[%s] Index location: %s", s.vaultID, s.indexPath)

	var err error

	// Try to open existing index
	s.index, err = bleve.Open(s.indexPath)
	if errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		// Create new index
		docMapping := buildIndexMapping()
		s.index, err = bleve.New(s.indexPath, docMapping)
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
		log.Printf("[%s] Created new index", s.vaultID)
	} else if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	} else {
		log.Printf("[%s] Opened existing index", s.vaultID)
	}

	// First, count total files
	totalFiles := 0
	err = filepath.WalkDir(s.vaultPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") && !strings.HasPrefix(filepath.Base(path), ".") {
			totalFiles++
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to count files: %w", err)
	}

	// Send initial status
	s.updateStatus(StatusUpdate{
		Status:         StatusInitialIndexing,
		TotalCount:     totalFiles,
		RemainingCount: totalFiles,
		IndexedCount:   0,
		Message:        fmt.Sprintf("Starting to index %d files", totalFiles),
	})

	// Walk through markdown files and index them
	batch := s.index.NewBatch()
	indexedCount := 0

	err = filepath.WalkDir(s.vaultPath, func(path string, d fs.DirEntry, err error) error {
		// Check for cancellation
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		doc, err := parseMarkdownFile(path)
		if err != nil {
			log.Printf("[%s] Error parsing %s: %v", s.vaultID, path, err)
			return nil // Continue processing other files
		}

		// Use relative path as document ID
		relPath, _ := filepath.Rel(s.vaultPath, path)
		err = batch.Index(relPath, doc)
		if err != nil {
			return err
		}
		indexedCount++

		// Batch index every 100 documents
		if batch.Size() >= 100 {
			if err := s.index.Batch(batch); err != nil {
				return fmt.Errorf("batch index failed: %w", err)
			}
			log.Printf("[%s] Indexed %d/%d documents...", s.vaultID, indexedCount, totalFiles)

			// Send progress update
			s.updateStatus(StatusUpdate{
				Status:         StatusInitialIndexing,
				TotalCount:     totalFiles,
				RemainingCount: totalFiles - indexedCount,
				IndexedCount:   indexedCount,
				Message:        fmt.Sprintf("Indexed %d/%d documents", indexedCount, totalFiles),
			})

			batch = s.index.NewBatch()
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk vault directory: %w", err)
	}

	// Index remaining documents
	if batch.Size() > 0 {
		if err := s.index.Batch(batch); err != nil {
			return fmt.Errorf("final batch index failed: %w", err)
		}
	}

	// Send final status
	s.updateStatus(StatusUpdate{
		Status:         StatusInitialIndexing,
		TotalCount:     totalFiles,
		RemainingCount: 0,
		IndexedCount:   indexedCount,
		Message:        fmt.Sprintf("Successfully indexed %d documents", indexedCount),
	})

	log.Printf("[%s] Successfully indexed %d documents", s.vaultID, indexedCount)

	// Notify search service that initial index is ready (rebuild event)
	s.notifyIndexUpdate("rebuild")

	return nil
}

// updateStatus sends a status update to the channel (non-blocking)
func (s *IndexService) updateStatus(update StatusUpdate) {
	select {
	case s.statusChan <- update:
	case <-s.ctx.Done():
	default:
		// Channel full, skip this update
	}
}

// StatusUpdates returns the read-only channel for status updates
func (s *IndexService) StatusUpdates() <-chan StatusUpdate {
	return s.statusChan
}

// GetStatus returns the current status of the service
func (s *IndexService) GetStatus() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// reIndex updates a single document in the index
// docPath should be a local file path (sync service provides this)
func (s *IndexService) reIndex(docPath string) error {
	if s.index == nil {
		return fmt.Errorf("index not initialized, call Index() first")
	}

	// Parse the markdown file
	doc, err := parseMarkdownFile(docPath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Use relative path as document ID
	relPath, err := filepath.Rel(s.vaultPath, docPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Index the document
	if err := s.index.Index(relPath, doc); err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}

	log.Printf("[%s] Re-indexed document: %s", s.vaultID, relPath)
	return nil
}

// deleteFromIndex removes a document from the index
// docPath should be a local file path (sync service provides this)
func (s *IndexService) deleteFromIndex(docPath string) error {
	if s.index == nil {
		return fmt.Errorf("index not initialized, call Index() first")
	}

	// Use relative path as document ID
	relPath, err := filepath.Rel(s.vaultPath, docPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	if err := s.index.Delete(relPath); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	log.Printf("[%s] Deleted document from index: %s", s.vaultID, relPath)
	return nil
}

// GetIndex returns the underlying bleve index
// This allows search operations to be performed
func (s *IndexService) GetIndex() bleve.Index {
	return s.index
}

// RegisterIndexNotifier registers a notifier to be called when the index is updated
func (s *IndexService) RegisterIndexNotifier(notifier IndexUpdateNotifier) {
	if notifier == nil {
		return
	}

	s.indexNotifiersMu.Lock()
	s.indexNotifiers = append(s.indexNotifiers, notifier)
	s.indexNotifiersMu.Unlock()

	log.Printf("[%s] Registered index update notifier", s.vaultID)
}

// notifyIndexUpdate notifies all registered notifiers that the index has been updated
func (s *IndexService) notifyIndexUpdate(eventType string) {
	s.indexNotifiersMu.RLock()
	notifiers := make([]IndexUpdateNotifier, len(s.indexNotifiers))
	copy(notifiers, s.indexNotifiers)
	s.indexNotifiersMu.RUnlock()

	if len(notifiers) == 0 {
		return
	}

	// Notify asynchronously to avoid blocking index operations
	go func() {
		event := IndexUpdateEvent{
			Timestamp: time.Now(),
			EventType: eventType,
		}

		// Only include index reference for rebuild events
		if eventType == "rebuild" {
			event.NewIndex = s.GetIndex()
		}

		for _, notifier := range notifiers {
			notifier.NotifyIndexUpdate(event)
		}
	}()
}

// processEvents processes index events from the sync service in the background
// with batching and event coalescing to handle backpressure
func (s *IndexService) processEvents() {
	defer s.wg.Done()

	log.Printf("[%s] Event processor started (buffer: %d, batch: %d, flush: %v)",
		s.vaultID, s.eventBuffer, s.batchSize, s.flushInterval)

	// Ticker for periodic batch flushing
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			// Flush any pending events before stopping
			s.flushPendingEvents()
			log.Printf("[%s] Event processor stopped (processed: %d, dropped: %d)",
				s.vaultID, atomic.LoadInt64(&s.processedEvents), atomic.LoadInt64(&s.droppedEvents))
			return

		case event, ok := <-s.eventChan:
			if !ok {
				// Channel closed, flush and exit
				s.flushPendingEvents()
				log.Printf("[%s] Event channel closed", s.vaultID)
				return
			}

			// Coalesce event (merge with existing event for same path)
			s.coalesceEvent(event)

			// Check if we should flush the batch
			s.pendingMu.Lock()
			pendingCount := len(s.pendingEvents)
			s.pendingMu.Unlock()

			if pendingCount >= s.batchSize {
				s.flushPendingEvents()
			}

		case <-ticker.C:
			// Periodic flush to avoid events sitting too long
			s.flushPendingEvents()
		}
	}
}

// coalesceEvent merges events for the same file path
// Latest event wins, reducing redundant processing
func (s *IndexService) coalesceEvent(event syncpkg.FileChangeEvent) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	existingEvent, exists := s.pendingEvents[event.Path]

	if exists {
		// Event coalescing rules:
		// 1. Delete always wins (if file deleted, ignore other events)
		// 2. Otherwise, latest event wins
		if existingEvent.EventType == syncpkg.FileDeleted {
			// Keep the delete event
			return
		}

		// Replace with newer event
		s.pendingEvents[event.Path] = event
	} else {
		// New path, add to pending
		s.pendingEvents[event.Path] = event
	}
}

// flushPendingEvents processes all coalesced events in a batch
func (s *IndexService) flushPendingEvents() {
	s.pendingMu.Lock()
	if len(s.pendingEvents) == 0 {
		s.pendingMu.Unlock()
		return
	}

	// Take ownership of pending events
	eventsToProcess := s.pendingEvents
	s.pendingEvents = make(map[string]syncpkg.FileChangeEvent)
	batchSize := len(eventsToProcess)
	s.pendingMu.Unlock()

	log.Printf("[%s] Flushing batch of %d coalesced events", s.vaultID, batchSize)

	// Process all events in the batch
	for _, event := range eventsToProcess {
		s.processEvent(event)
		atomic.AddInt64(&s.processedEvents, 1)
	}

	// Notify search service that index has been updated incrementally
	s.notifyIndexUpdate("incremental")
}

// processEvent handles a single sync event
func (s *IndexService) processEvent(event syncpkg.FileChangeEvent) {
	// Only process events when index is ready
	s.mu.RLock()
	status := s.status
	s.mu.RUnlock()

	if status != StatusReady {
		log.Printf("[%s] Skipping event %s for %s - service not ready (status: %s)",
			s.vaultID, event.EventType, event.Path, status)
		return
	}

	switch event.EventType {
	case syncpkg.FileCreated, syncpkg.FileModified:
		if err := s.reIndex(event.Path); err != nil {
			log.Printf("[%s] Failed to re-index %s: %v", s.vaultID, event.Path, err)
		} else {
			log.Printf("[%s] Re-indexed %s (%s)", s.vaultID, event.Path, event.EventType)
		}

	case syncpkg.FileDeleted:
		if err := s.deleteFromIndex(event.Path); err != nil {
			log.Printf("[%s] Failed to delete %s from index: %v", s.vaultID, event.Path, err)
		} else {
			log.Printf("[%s] Deleted %s from index", s.vaultID, event.Path)
		}

	default:
		log.Printf("[%s] Unknown event type: %s for %s", s.vaultID, event.EventType, event.Path)
	}
}

// UpdateIndex sends a sync event to be processed (non-blocking)
// This is called by the sync service when files are created, modified, or deleted
// The sync service passes its FileChangeEvent directly to this method
func (s *IndexService) UpdateIndex(event syncpkg.FileChangeEvent) {
	// Validate event
	if event.Path == "" {
		log.Printf("[%s] Warning: Ignoring event with empty path", s.vaultID)
		return
	}

	// Validate event type
	switch event.EventType {
	case syncpkg.FileCreated, syncpkg.FileModified, syncpkg.FileDeleted:
		// Valid event type, continue
	default:
		log.Printf("[%s] Warning: Unknown event type %s for %s", s.vaultID, event.EventType, event.Path)
		return
	}

	// Queue event for async processing
	select {
	case s.eventChan <- event:
		// Event queued successfully for processing
		// Note: Verbose logging disabled for performance - enable for debugging
		// log.Printf("[%s] Queued %s event for: %s", s.vaultID, event.EventType, event.Path)
	case <-s.ctx.Done():
		// Context cancelled, skip event
		log.Printf("[%s] Context cancelled, skipping event for: %s", s.vaultID, event.Path)
		atomic.AddInt64(&s.droppedEvents, 1)
	default:
		// Channel full, drop event and track metric
		atomic.AddInt64(&s.droppedEvents, 1)
		dropped := atomic.LoadInt64(&s.droppedEvents)

		// Log warning periodically to avoid log spam
		if dropped%100 == 1 || dropped < 10 {
			log.Printf("[%s] ⚠️  BACKPRESSURE: Event channel full (%d buffer), dropped %d events total. "+
				"Consider increasing buffer size or reducing event rate.",
				s.vaultID, s.eventBuffer, dropped)
		}
	}
}

// Stop stops the indexing service gracefully
func (s *IndexService) Stop() error {
	// Cancel the context to signal goroutines to stop
	s.cancel()

	// Close the event channel to signal event processor to exit
	close(s.eventChan)

	// Wait for all goroutines to finish
	s.wg.Wait()

	// Close the index
	if s.index != nil {
		log.Printf("[%s] Closing index", s.vaultID)
		if err := s.index.Close(); err != nil {
			return fmt.Errorf("failed to close index: %w", err)
		}
	}

	s.mu.Lock()
	s.status = StatusStopped
	s.mu.Unlock()

	return nil
}

// Close is an alias for Stop for backward compatibility
func (s *IndexService) Close() error {
	return s.Stop()
}

// VaultID returns the vault ID
func (s *IndexService) VaultID() string {
	return s.vaultID
}

// VaultName returns the vault name
func (s *IndexService) VaultName() string {
	return s.vaultName
}

// IndexMetrics provides metrics about the indexing service
type IndexMetrics struct {
	ProcessedEvents int64 // Total events successfully processed
	DroppedEvents   int64 // Total events dropped due to backpressure
	PendingEvents   int   // Current number of pending events
	BufferSize      int   // Event channel buffer size
	BatchSize       int   // Configured batch size
	FlushInterval   time.Duration
}

// GetMetrics returns current indexing metrics
func (s *IndexService) GetMetrics() IndexMetrics {
	s.pendingMu.Lock()
	pending := len(s.pendingEvents)
	s.pendingMu.Unlock()

	return IndexMetrics{
		ProcessedEvents: atomic.LoadInt64(&s.processedEvents),
		DroppedEvents:   atomic.LoadInt64(&s.droppedEvents),
		PendingEvents:   pending,
		BufferSize:      s.eventBuffer,
		BatchSize:       s.batchSize,
		FlushInterval:   s.flushInterval,
	}
}
