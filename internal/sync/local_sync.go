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

			// Filter for markdown files only
			if !l.isMarkdownFile(event.Name) {
				continue
			}

			// Handle directory creation (need to watch new directories)
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					// New directory created, add it to watcher recursively
					_ = l.addRecursive(event.Name)
				}
			}

			// Convert fsnotify event to FileChangeEvent
			fileEvent := l.convertEvent(event)
			if fileEvent != nil {
				// Send event non-blocking
				select {
				case events <- *fileEvent:
				case <-ctx.Done():
					return
				default:
					// Channel full, skip this event
					logger.WithFields(map[string]interface{}{
						"vault_id": l.vaultID,
						"path":     event.Name,
						"event":    fileEvent.EventType.String(),
					}).Warn("Event channel full, dropping event")
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

// isMarkdownFile checks if the file is a markdown file
func (l *localSync) isMarkdownFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md"
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
