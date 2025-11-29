package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/susamn/obsidian-web/internal/logger"
)

// localSync monitors local filesystem for changes using fsnotify
type localSync struct {
	vaultID  string
	rootPath string
	watcher  *fsnotify.Watcher
}

// newLocalSync creates a new local filesystem sync service
func newLocalSync(vaultID, rootPath string) (*localSync, error) {
	// Validate root path exists
	if _, err := os.Stat(rootPath); err != nil {
		return nil, fmt.Errorf("vault path does not exist: %w", err)
	}

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	return &localSync{
		vaultID:  vaultID,
		rootPath: rootPath,
		watcher:  watcher,
	}, nil
}

// Start begins monitoring the filesystem in a non-blocking manner
func (l *localSync) Start(ctx context.Context, events chan<- FileChangeEvent) error {
	logger.WithFields(map[string]interface{}{
		"vault_id": l.vaultID,
		"path":     l.rootPath,
	}).Info("Starting local filesystem sync")

	// Add all directories recursively to the watcher
	if err := l.addRecursive(l.rootPath); err != nil {
		return fmt.Errorf("failed to add directories to watcher: %w", err)
	}

	// Start the event loop in the current goroutine (will block until context is cancelled)
	l.watchLoop(ctx, events)

	logger.WithField("vault_id", l.vaultID).Info("Local sync stopped")
	return nil
}

// Stop stops the filesystem watcher
func (l *localSync) Stop() error {
	if l.watcher != nil {
		return l.watcher.Close()
	}
	return nil
}

// ReIndex walks the entire vault and emits FileCreated events for all files
func (l *localSync) ReIndex(events chan<- FileChangeEvent) error {
	logger.WithField("vault_id", l.vaultID).Info("Starting local re-index")

	// Run in a separate goroutine to avoid blocking
	go func() {
		// Context for re-indexing (can be cancelled if needed, but here we use Background)
		// In a real implementation, we might want to pass a context from SyncService
		ctx := context.Background()

		// Reuse emitEventsForDirectory to walk the root and emit events
		l.emitEventsForDirectory(ctx, l.rootPath, events, FileCreated)

		logger.WithField("vault_id", l.vaultID).Info("Local re-index walk completed")
	}()

	return nil
}

// watchLoop processes filesystem events (blocking)
func (l *localSync) watchLoop(ctx context.Context, events chan<- FileChangeEvent) {
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop watching
			return

		case event, ok := <-l.watcher.Events:
			if !ok {
				// Watcher closed
				return
			}

			// Check if this is a directory
			info, statErr := os.Stat(event.Name)
			isDir := statErr == nil && info.IsDir()

			// Handle directory creation (need to watch new directories and emit events for all contents)
			if event.Op&fsnotify.Create != 0 && isDir {
				// New directory created, add it to watcher recursively
				_ = l.addRecursive(event.Name)

				// Emit FileCreated events for all files inside the newly created directory
				l.emitEventsForDirectory(ctx, event.Name, events, FileCreated)
				continue
			}

			// Skip hidden files (but allow all other file types)
			if l.isHiddenFile(event.Name) {
				continue
			}

			// Skip directories (already handled above)
			if isDir {
				continue
			}

			// Convert fsnotify event to FileChangeEvent
			fileEvent := l.convertEvent(event)
			if fileEvent != nil {
				// Send event BLOCKING - let backpressure propagate to fsnotify
				// This prevents event loss during bulk operations
				select {
				case events <- *fileEvent:
					// Event sent successfully
				case <-ctx.Done():
					return
				}
			}

		case err, ok := <-l.watcher.Errors:
			if !ok {
				// Watcher closed
				return
			}
			// Log watcher error
			logger.WithError(err).WithField("vault_id", l.vaultID).Error("Filesystem watcher error")
		}
	}
}

// addRecursive adds a directory and all subdirectories to the watcher
func (l *localSync) addRecursive(path string) error {
	dirCount := 0
	err := filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only watch directories
		if info.IsDir() {
			// Skip hidden directories (like .git, .obsidian)
			if l.isHiddenDir(walkPath) && walkPath != l.rootPath {
				return filepath.SkipDir
			}

			if err := l.watcher.Add(walkPath); err != nil {
				return fmt.Errorf("failed to watch directory %s: %w", walkPath, err)
			}
			dirCount++
		}

		return nil
	})

	if err == nil && dirCount > 0 {
		logger.WithFields(map[string]interface{}{
			"vault_id":    l.vaultID,
			"directories": dirCount,
			"root":        path,
		}).Debug("Added directories to watcher")
	}

	return err
}

// emitEventsForDirectory walks a directory and emits events for all files
func (l *localSync) emitEventsForDirectory(ctx context.Context, dirPath string, events chan<- FileChangeEvent, eventType FileEventType) {
	err := filepath.Walk(dirPath, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files and directories
		if l.isHiddenDir(walkPath) && walkPath != dirPath {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Process all files (not directories)
		if !info.IsDir() && !l.isHiddenFile(walkPath) {
			fileEvent := &FileChangeEvent{
				VaultID:   l.vaultID,
				Path:      walkPath,
				EventType: eventType,
				Timestamp: time.Now(),
			}

			// Send event BLOCKING - ensures all files in new directory are tracked
			select {
			case events <- *fileEvent:
				logger.WithFields(map[string]interface{}{
					"vault_id": l.vaultID,
					"path":     walkPath,
					"event":    eventType.String(),
				}).Debug("Emitted event for file in newly created directory")
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return nil
	})

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"vault_id": l.vaultID,
			"path":     dirPath,
		}).WithError(err).Warn("Failed to emit events for directory contents")
	}
}

// isMarkdownFile checks if the file is a markdown file (kept for potential future use)
func (l *localSync) isMarkdownFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md"
}

// isHiddenFile checks if a file is hidden (starts with .)
func (l *localSync) isHiddenFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".")
}

// isHiddenDir checks if a directory is hidden (starts with .)
func (l *localSync) isHiddenDir(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".")
}

// convertEvent converts fsnotify event to FileChangeEvent
func (l *localSync) convertEvent(event fsnotify.Event) *FileChangeEvent {
	var eventType FileEventType

	switch {
	case event.Op&fsnotify.Create != 0:
		eventType = FileCreated
	case event.Op&fsnotify.Write != 0:
		eventType = FileModified
	case event.Op&fsnotify.Remove != 0:
		eventType = FileDeleted
	case event.Op&fsnotify.Rename != 0:
		// Treat rename as delete (the old path is gone)
		eventType = FileDeleted
	default:
		// Ignore other events (chmod, etc.)
		return nil
	}

	return &FileChangeEvent{
		VaultID:   l.vaultID,
		Path:      event.Name,
		EventType: eventType,
		Timestamp: time.Now(),
	}
}
