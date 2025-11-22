package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/susamn/obsidian-web/internal/db"
	"github.com/susamn/obsidian-web/internal/logger"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
	"github.com/susamn/obsidian-web/internal/utils"
)

// Global mutex for parent directory creation to prevent race conditions
// when multiple workers try to create the same parent path simultaneously
var parentDirMutex sync.Mutex

// performDatabaseUpdate updates the database for a given file event
// This is a shared helper that both vault.updateDatabase and worker.updateDatabase use
// Returns the file ID and error
// For create/modify events, returns the ID of the created/updated file
// For delete events, returns the ID of the deleted file (before deletion)
func performDatabaseUpdate(dbService *db.DBService, vaultPath string, event syncpkg.FileChangeEvent) (string, error) {
	if dbService == nil {
		return "", fmt.Errorf("db service not available")
	}

	// Convert absolute path to relative path
	relPath, err := filepath.Rel(vaultPath, event.Path)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	switch event.EventType {
	case syncpkg.FileCreated, syncpkg.FileModified:
		// Determine if it's a directory and get file info
		isDir := false
		var size int64
		if info, err := os.Stat(event.Path); err == nil {
			isDir = info.IsDir()
			if !isDir {
				size = info.Size()
			}
		}

		// Detect file type
		fileType := db.DetectFileType(filepath.Base(event.Path), isDir)
		fileTypeID, err := dbService.GetFileTypeID(fileType)
		if err != nil {
			logger.WithField("file_type", fileType).WithField("error", err).Warn("Failed to get file type ID")
		}

		// Ensure parent directories exist in the database
		var parentID *string
		parentPath := filepath.Dir(relPath)
		if parentPath != "." && parentPath != "" {
			// Item is nested, ensure parent directories exist
			parentID = ensureParentDirsExist(dbService, vaultPath, parentPath)
		} else {
			// Item is at root level, set parent to root node ID
			rootEntry, err := dbService.GetFileEntryByPath("")
			if err == nil && rootEntry != nil {
				// Root exists, use its ID as parent
				parentID = &rootEntry.ID
			}
		}

		// Create or update file entry in database
		entry := &db.FileEntry{
			ID:         utils.GenerateID(),
			Name:       filepath.Base(event.Path),
			IsDir:      isDir,
			FileTypeID: fileTypeID,
			Created:    event.Timestamp,
			Modified:   event.Timestamp,
			Size:       size,
			Path:       relPath,
			ParentID:   parentID,
		}

		// Check if entry already exists
		existing, err := dbService.GetFileEntryByPath(relPath)
		if err == nil && existing != nil {
			// Update existing entry
			entry.ID = existing.ID
			entry.Created = existing.Created
			entry.ParentID = existing.ParentID
			entry.FileTypeID = fileTypeID
			if err := dbService.UpdateFileEntry(entry); err != nil {
				return "", fmt.Errorf("failed to update entry: %w", err)
			}
			return entry.ID, nil
		} else {
			// Create new entry
			if err := dbService.CreateFileEntry(entry); err != nil {
				return "", fmt.Errorf("failed to create entry: %w", err)
			}
			return entry.ID, nil
		}

	case syncpkg.FileDeleted:
		// Delete entry from database
		entry, err := dbService.GetFileEntryByPath(relPath)
		if err == nil && entry != nil {
			fileID := entry.ID
			if err := dbService.DeleteFileEntry(entry.ID); err != nil {
				return "", fmt.Errorf("failed to delete entry: %w", err)
			}
			return fileID, nil
		}
		return "", nil // File not found in DB, but not an error
	}

	return "", nil
}

// ensureParentDirsExist ensures all parent directories exist in the database and returns the ID of the immediate parent
func ensureParentDirsExist(dbService *db.DBService, vaultPath, parentPath string) *string {
	// Lock to prevent race conditions when multiple workers create same parent dirs
	parentDirMutex.Lock()
	defer parentDirMutex.Unlock()

	// Ensure root directory exists first
	rootEntry, err := dbService.GetFileEntryByPath("")
	var currentParentID *string
	if err != nil || rootEntry == nil {
		// Root doesn't exist, create it
		rootID := utils.GenerateID()
		dirFileTypeID, _ := dbService.GetFileTypeID(db.FileTypeDirectory)
		rootEntry := &db.FileEntry{
			ID:         rootID,
			Name:       "vault",
			ParentID:   nil,
			IsDir:      true,
			FileTypeID: dirFileTypeID,
			Path:       "",
			Created:    time.Now().UTC(),
			Modified:   time.Now().UTC(),
		}
		if err := dbService.CreateFileEntry(rootEntry); err != nil {
			// Duplicate key error is expected when multiple workers try to create root
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				logger.WithField("error", err).Warn("Failed to create root directory in database")
			}
			// Try to fetch it again - it was created by another worker
			if rootEntry2, err := dbService.GetFileEntryByPath(""); err == nil && rootEntry2 != nil {
				id := rootEntry2.ID
				currentParentID = &id
			}
		} else {
			currentParentID = &rootID
		}
	} else {
		id := rootEntry.ID
		currentParentID = &id
	}

	// Split the path into components
	parts := strings.Split(filepath.Clean(parentPath), string(filepath.Separator))

	currentPath := ""

	// Create each directory in the hierarchy
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = filepath.Join(currentPath, part)
		}

		// Check if this directory exists in the database
		existing, err := dbService.GetFileEntryByPath(currentPath)
		if err == nil && existing != nil {
			// Directory already exists, update the parent ID for next iteration
			id := existing.ID
			currentParentID = &id
			continue
		}

		// Directory doesn't exist, create it
		dirFileTypeID, _ := dbService.GetFileTypeID(db.FileTypeDirectory)
		dirEntry := &db.FileEntry{
			ID:         utils.GenerateID(),
			Name:       part,
			IsDir:      true,
			FileTypeID: dirFileTypeID,
			ParentID:   currentParentID,
			Created:    time.Now().UTC(),
			Modified:   time.Now().UTC(),
			Path:       currentPath,
		}

		if err := dbService.CreateFileEntry(dirEntry); err != nil {
			// Duplicate key error is expected when multiple workers try to create same parent
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				logger.WithField("path", currentPath).WithField("error", err).Warn("Failed to create parent directory in database")
			}
			// Directory was created by another worker, fetch it
			if existing2, err := dbService.GetFileEntryByPath(currentPath); err == nil && existing2 != nil {
				id := existing2.ID
				currentParentID = &id
			}
			continue
		}

		// Update parent ID for next iteration
		id := dirEntry.ID
		currentParentID = &id
	}

	return currentParentID
}
