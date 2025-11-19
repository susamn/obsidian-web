package explorer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/susamn/obsidian-web/internal/db"
	"github.com/susamn/obsidian-web/internal/logger"
	"github.com/susamn/obsidian-web/internal/sse"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
)

// NodeType represents the type of file system node
type NodeType string

const (
	NodeTypeFile      NodeType = "file"
	NodeTypeDirectory NodeType = "directory"
)

// NodeMetadata represents metadata about a file or directory
type NodeMetadata struct {
	ID          string    `json:"id"`            // Unique identifier from database
	Path        string    `json:"-"`             // Relative path from vault root (internal only)
	Name        string    `json:"name"`          // File/directory name
	Type        NodeType  `json:"type"`          // file or directory
	IsDirectory bool      `json:"is_directory"`  // True if directory (convenience field)
	Size        int64     `json:"size"`          // Size in bytes (0 for directories)
	ModTime     time.Time `json:"modified_time"` // Last modification time
	IsMarkdown  bool      `json:"is_markdown"`   // True if .md file
	HasChildren bool      `json:"has_children"`  // True if directory has children
	ChildCount  int       `json:"child_count"`   // Number of direct children
	CachedAt    time.Time `json:"-"`             // When this was cached
}

// TreeNode represents a node in the directory tree with lazy-loaded children
type TreeNode struct {
	Metadata NodeMetadata `json:"metadata"`
	Children []*TreeNode  `json:"children,omitempty"` // Populated on demand
	Loaded   bool         `json:"-"`                  // Whether children have been loaded
}

// SSEBroadcaster defines the interface for SSE broadcasting
type SSEBroadcaster interface {
	BroadcastFileEvent(vaultID, path string, eventType interface{})
}

// SSEBroadcasterWithData is an interface for SSE broadcasting with rich metadata
// The concrete sse.Manager implements both BroadcastFileEvent and BroadcastFileEventWithData
type SSEBroadcasterWithData interface {
	BroadcastFileEvent(vaultID, path string, eventType interface{})
	BroadcastFileEventWithData(vaultID, path string, eventType sse.EventType, fileData *sse.FileEventData)
}

// ExplorerService provides lazy-loaded directory tree exploration with caching
type ExplorerService struct {
	ctx       context.Context
	cancel    context.CancelFunc
	vaultID   string
	vaultPath string // Base directory for the vault

	// Cache: path -> TreeNode
	cache   map[string]*TreeNode
	cacheMu sync.RWMutex

	// Event handling
	eventChan chan syncpkg.FileChangeEvent
	wg        sync.WaitGroup

	// SSE broadcasting
	sseBroadcaster SSEBroadcaster

	// Database service for storing file metadata
	dbService *db.DBService

	// Configuration
	maxCacheSize int           // Maximum number of cached nodes
	cacheTTL     time.Duration // Time to live for cache entries
}

// NewExplorerService creates a new explorer service
func NewExplorerService(ctx context.Context, vaultID, vaultPath string, dbSvc *db.DBService) (*ExplorerService, error) {
	if vaultPath == "" {
		return nil, fmt.Errorf("vault path cannot be empty")
	}

	// Validate base directory exists
	if _, err := os.Stat(vaultPath); err != nil {
		return nil, fmt.Errorf("vault path does not exist: %w", err)
	}

	svcCtx, cancel := context.WithCancel(ctx)

	return &ExplorerService{
		ctx:          svcCtx,
		cancel:       cancel,
		vaultID:      vaultID,
		vaultPath:    vaultPath,
		cache:        make(map[string]*TreeNode),
		eventChan:    make(chan syncpkg.FileChangeEvent, 100),
		dbService:    dbSvc,
		maxCacheSize: 1000,
		cacheTTL:     5 * time.Minute,
	}, nil
}

// Start starts the explorer service
func (e *ExplorerService) Start() error {
	logger.WithField("vault_id", e.vaultID).Info("Starting explorer service")

	// Start event processor
	e.wg.Add(1)
	go e.processEvents()

	return nil
}

// Stop stops the explorer service
func (e *ExplorerService) Stop() error {
	logger.WithField("vault_id", e.vaultID).Info("Stopping explorer service")

	e.cancel()
	close(e.eventChan)
	e.wg.Wait()

	return nil
}

// GetTree returns the directory tree for a given path with children loaded
// If path is empty, returns the root of the vault
func (e *ExplorerService) GetTree(path string) (*TreeNode, error) {
	// Sanitize and validate path
	cleanPath, err := e.validatePath(path)
	if err != nil {
		return nil, err
	}

	// Check cache first
	e.cacheMu.RLock()
	node, exists := e.cache[cleanPath]
	e.cacheMu.RUnlock()

	if exists && !e.isCacheExpired(node) && node.Loaded {
		// Cache hit with children loaded - return cached node
		logger.WithFields(map[string]interface{}{
			"vault_id": e.vaultID,
			"path":     cleanPath,
		}).Debug("Cache hit for tree node with children")
		return node, nil
	}

	// Cache miss or expired - scan directory
	node, err = e.scanDirectory(cleanPath)
	if err != nil {
		return nil, err
	}

	// Load children immediately for tree endpoint
	if node.Metadata.Type == NodeTypeDirectory {
		if err := e.loadChildren(node); err != nil {
			return nil, err
		}
	}

	// Update cache
	e.updateCache(cleanPath, node)

	return node, nil
}

// GetChildren returns just the children of a directory (lazy load)
func (e *ExplorerService) GetChildren(path string) ([]*TreeNode, error) {
	node, err := e.GetTree(path)
	if err != nil {
		return nil, err
	}

	if node.Metadata.Type != NodeTypeDirectory {
		return nil, fmt.Errorf("path is not a directory")
	}

	// Load children if not already loaded
	if !node.Loaded {
		if err := e.loadChildren(node); err != nil {
			return nil, err
		}
	}

	return node.Children, nil
}

// GetMetadata returns metadata for a file or directory
func (e *ExplorerService) GetMetadata(path string) (*NodeMetadata, error) {
	cleanPath, err := e.validatePath(path)
	if err != nil {
		return nil, err
	}

	fullPath := e.buildFullPath(cleanPath)
	return e.getNodeMetadata(fullPath, cleanPath)
}

// UpdateIndex handles file change events from sync service
func (e *ExplorerService) UpdateIndex(event syncpkg.FileChangeEvent) {
	select {
	case e.eventChan <- event:
		// Event queued successfully
	case <-e.ctx.Done():
		// Context cancelled
	default:
		// Channel full, drop event
		logger.WithField("vault_id", e.vaultID).Warn("Explorer event channel full")
	}
}

// validatePath sanitizes and validates a path to prevent directory traversal
func (e *ExplorerService) validatePath(path string) (string, error) {
	// Clean the path
	cleanPath := filepath.Clean(path)

	// Handle empty path (root)
	if cleanPath == "." || cleanPath == "" {
		return "", nil
	}

	// Prevent directory traversal
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("invalid path: directory traversal not allowed")
	}

	// Ensure path doesn't start with /
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	// Build full path and ensure it's within vault
	fullPath := e.buildFullPath(cleanPath)
	absVaultPath, err := filepath.Abs(e.vaultPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute vault path: %w", err)
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Ensure requested path is within vault directory
	if !strings.HasPrefix(absFullPath, absVaultPath) {
		return "", fmt.Errorf("path is outside vault directory")
	}

	return cleanPath, nil
}

// buildFullPath constructs the full filesystem path
func (e *ExplorerService) buildFullPath(relativePath string) string {
	if relativePath == "" {
		return e.vaultPath
	}
	return filepath.Join(e.vaultPath, relativePath)
}

// scanDirectory scans a directory and creates a TreeNode
func (e *ExplorerService) scanDirectory(relativePath string) (*TreeNode, error) {
	fullPath := e.buildFullPath(relativePath)

	// Get metadata for the node itself
	metadata, err := e.getNodeMetadata(fullPath, relativePath)
	if err != nil {
		return nil, err
	}

	node := &TreeNode{
		Metadata: *metadata,
		Children: nil,
		Loaded:   false,
	}

	return node, nil
}

// loadChildren loads the children of a directory node
func (e *ExplorerService) loadChildren(node *TreeNode) error {
	if node.Metadata.Type != NodeTypeDirectory {
		return fmt.Errorf("cannot load children of non-directory")
	}

	fullPath := e.buildFullPath(node.Metadata.Path)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	children := make([]*TreeNode, 0, len(entries))
	for _, entry := range entries {
		// Skip hidden files/directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		childPath := filepath.Join(node.Metadata.Path, entry.Name())
		childFullPath := e.buildFullPath(childPath)

		metadata, err := e.getNodeMetadata(childFullPath, childPath)
		if err != nil {
			logger.WithError(err).WithFields(map[string]interface{}{
				"vault_id": e.vaultID,
				"path":     childPath,
			}).Warn("Failed to get child metadata")
			continue
		}

		child := &TreeNode{
			Metadata: *metadata,
			Children: nil,
			Loaded:   false,
		}

		children = append(children, child)
	}

	node.Children = children
	node.Loaded = true
	node.Metadata.ChildCount = len(children)
	node.Metadata.HasChildren = len(children) > 0

	return nil
}

// getNodeMetadata gets metadata for a file or directory
func (e *ExplorerService) getNodeMetadata(fullPath, relativePath string) (*NodeMetadata, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	nodeType := NodeTypeFile
	hasChildren := false
	childCount := 0

	if info.IsDir() {
		nodeType = NodeTypeDirectory

		// Check if directory has children (quick check)
		entries, err := os.ReadDir(fullPath)
		if err == nil {
			// Count non-hidden entries
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), ".") {
					childCount++
				}
			}
			hasChildren = childCount > 0
		}
	}

	isMarkdown := false
	if nodeType == NodeTypeFile {
		isMarkdown = strings.HasSuffix(strings.ToLower(info.Name()), ".md")
	}

	isDirectory := nodeType == NodeTypeDirectory

	// Fetch ID from database if available
	id := ""
	if e.dbService != nil {
		entry, err := e.dbService.GetFileEntryByPath(relativePath)
		if err == nil && entry != nil {
			id = entry.ID
		}
	}

	return &NodeMetadata{
		ID:          id,
		Path:        relativePath,
		Name:        info.Name(),
		Type:        nodeType,
		IsDirectory: isDirectory,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		IsMarkdown:  isMarkdown,
		HasChildren: hasChildren,
		ChildCount:  childCount,
		CachedAt:    time.Now(),
	}, nil
}

// updateCache updates the cache with a node
func (e *ExplorerService) updateCache(path string, node *TreeNode) {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()

	// Check cache size and evict if necessary
	if len(e.cache) >= e.maxCacheSize {
		e.evictOldestEntries(10) // Evict 10% of cache
	}

	e.cache[path] = node

	logger.WithFields(map[string]interface{}{
		"vault_id":   e.vaultID,
		"path":       path,
		"cache_size": len(e.cache),
	}).Debug("Updated cache")
}

// invalidateCache removes a path from cache
func (e *ExplorerService) invalidateCache(path string) {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()

	delete(e.cache, path)

	logger.WithFields(map[string]interface{}{
		"vault_id": e.vaultID,
		"path":     path,
	}).Debug("Invalidated cache entry")
}

// invalidateParent invalidates the parent directory cache
func (e *ExplorerService) invalidateParent(path string) {
	parentPath := filepath.Dir(path)
	if parentPath == "." {
		parentPath = ""
	}

	e.invalidateCache(parentPath)
}

// isCacheExpired checks if a cache entry is expired
func (e *ExplorerService) isCacheExpired(node *TreeNode) bool {
	if e.cacheTTL == 0 {
		return false // No expiry
	}
	return time.Since(node.Metadata.CachedAt) > e.cacheTTL
}

// evictOldestEntries removes the oldest N entries from cache
func (e *ExplorerService) evictOldestEntries(count int) {
	if len(e.cache) == 0 {
		return
	}

	// Collect entries with timestamps
	type entry struct {
		path      string
		timestamp time.Time
	}

	entries := make([]entry, 0, len(e.cache))
	for path, node := range e.cache {
		entries = append(entries, entry{
			path:      path,
			timestamp: node.Metadata.CachedAt,
		})
	}

	// Sort by timestamp (oldest first)
	// Simple bubble sort for small counts
	for i := 0; i < len(entries)-1; i++ {
		for j := 0; j < len(entries)-i-1; j++ {
			if entries[j].timestamp.After(entries[j+1].timestamp) {
				entries[j], entries[j+1] = entries[j+1], entries[j]
			}
		}
	}

	// Remove oldest entries
	evictCount := count
	if evictCount > len(entries) {
		evictCount = len(entries)
	}

	for i := 0; i < evictCount; i++ {
		delete(e.cache, entries[i].path)
	}

	logger.WithFields(map[string]interface{}{
		"vault_id": e.vaultID,
		"evicted":  evictCount,
	}).Debug("Evicted old cache entries")
}

// processEvents handles file change events
func (e *ExplorerService) processEvents() {
	defer e.wg.Done()

	logger.WithField("vault_id", e.vaultID).Info("Explorer event processor started")

	for {
		select {
		case <-e.ctx.Done():
			logger.WithField("vault_id", e.vaultID).Info("Explorer event processor stopped")
			return

		case event, ok := <-e.eventChan:
			if !ok {
				logger.WithField("vault_id", e.vaultID).Info("Explorer event channel closed")
				return
			}

			e.handleFileEvent(event)
		}
	}
}

// handleFileEvent processes a single file change event and updates cache
func (e *ExplorerService) handleFileEvent(event syncpkg.FileChangeEvent) {
	// Convert absolute path to relative path
	relPath, err := filepath.Rel(e.vaultPath, event.Path)
	if err != nil {
		logger.WithError(err).WithField("vault_id", e.vaultID).Warn("Failed to get relative path")
		return
	}

	logger.WithFields(map[string]interface{}{
		"vault_id":   e.vaultID,
		"path":       relPath,
		"event_type": event.EventType,
	}).Debug("Processing file event")

	var sseEventType string
	var parentPath string

	switch event.EventType {
	case syncpkg.FileCreated:
		// Invalidate parent (child list changed)
		parentPath = filepath.Dir(relPath)
		if parentPath == "." {
			parentPath = ""
		}
		e.invalidateCache(parentPath)
		// Refresh parent cache with new child
		if parent, err := e.GetTree(parentPath); err == nil && parent != nil {
			e.updateCache(parentPath, parent)
		}
		sseEventType = "file_created"

	case syncpkg.FileModified:
		// Invalidate the node itself
		e.invalidateCache(relPath)
		// Get parent path for UI update
		parentPath = filepath.Dir(relPath)
		if parentPath == "." {
			parentPath = ""
		}
		sseEventType = "file_modified"

	case syncpkg.FileDeleted:
		// Invalidate the node itself
		e.invalidateCache(relPath)
		// Invalidate parent (child list changed)
		parentPath = filepath.Dir(relPath)
		if parentPath == "." {
			parentPath = ""
		}
		e.invalidateCache(parentPath)
		// Refresh parent cache
		if parent, err := e.GetTree(parentPath); err == nil && parent != nil {
			e.updateCache(parentPath, parent)
		}
		sseEventType = "file_deleted"
	}

	// Broadcast SSE event with rich metadata if broadcaster is available
	if e.sseBroadcaster != nil && sseEventType != "" {
		// Build the event data as a map that can be sent with the event
		eventData := map[string]interface{}{
			"path":       relPath,
			"event_type": sseEventType,
		}

		// Add file metadata if available
		if event.Path != "" {
			fileData := e.buildFileEventDataSSE(relPath, parentPath, event.Path, event.EventType)
			if fileData != nil {
				eventData["file_data"] = fileData
			}
		}

		// Broadcast the event
		e.sseBroadcaster.BroadcastFileEvent(e.vaultID, relPath, sseEventType)
	}
}

// buildFileEventData constructs rich metadata for SSE events (legacy interface{} version)
func (e *ExplorerService) buildFileEventData(relativePath, parentPath, fullPath string) interface{} {
	info, err := os.Stat(fullPath)
	if err != nil {
		// File may have been deleted, return minimal data
		return map[string]interface{}{
			"name":        filepath.Base(relativePath),
			"parent_path": parentPath,
		}
	}

	isMarkdown := false
	if !info.IsDir() {
		isMarkdown = strings.HasSuffix(strings.ToLower(info.Name()), ".md")
	}

	return map[string]interface{}{
		"name":        info.Name(),
		"is_dir":      info.IsDir(),
		"is_markdown": isMarkdown,
		"parent_path": parentPath,
		"size":        info.Size(),
		"mod_time":    info.ModTime().Unix(),
	}
}

// buildFileEventDataSSE constructs rich metadata for SSE events (SSE type version)
func (e *ExplorerService) buildFileEventDataSSE(relativePath, parentPath, fullPath string, eventType syncpkg.FileEventType) *sse.FileEventData {
	fileData := &sse.FileEventData{
		Name:       filepath.Base(relativePath),
		ParentPath: parentPath,
	}

	// For deleted files, we may not be able to stat them
	info, err := os.Stat(fullPath)
	if err != nil {
		// File was deleted or doesn't exist, return minimal data
		fileData.IsDir = false
		fileData.Size = 0
		fileData.ModTime = 0
		return fileData
	}

	fileData.IsDir = info.IsDir()
	fileData.Size = info.Size()
	fileData.ModTime = info.ModTime().Unix()

	if !info.IsDir() {
		fileData.IsMarkdown = strings.HasSuffix(strings.ToLower(info.Name()), ".md")
	}

	return fileData
}

// RefreshPath manually refreshes a directory subtree
func (e *ExplorerService) RefreshPath(path string) error {
	cleanPath, err := e.validatePath(path)
	if err != nil {
		return err
	}

	// Invalidate cache
	e.invalidateCache(cleanPath)

	// Rescan
	_, err = e.GetTree(cleanPath)
	return err
}

// ClearCache clears all cached entries
func (e *ExplorerService) ClearCache() {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()

	e.cache = make(map[string]*TreeNode)

	logger.WithField("vault_id", e.vaultID).Info("Cleared explorer cache")
}

// GetCacheStats returns cache statistics
func (e *ExplorerService) GetCacheStats() map[string]interface{} {
	e.cacheMu.RLock()
	defer e.cacheMu.RUnlock()

	return map[string]interface{}{
		"size":     len(e.cache),
		"max_size": e.maxCacheSize,
		"ttl":      e.cacheTTL.String(),
		"vault_id": e.vaultID,
	}
}

// SetSSEBroadcaster sets the SSE broadcaster for real-time updates
func (e *ExplorerService) SetSSEBroadcaster(broadcaster SSEBroadcaster) {
	e.sseBroadcaster = broadcaster
}
