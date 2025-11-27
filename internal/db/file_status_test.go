package db

import (
	"context"
	"path/filepath"
	"testing"
)

// TestFileStatus tests the file status functionality
func TestFileStatus(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	svc, err := NewDBService(context.Background(), &dbPath)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start DBService: %v", err)
	}
	defer svc.Stop()

	// Test 1: Verify all statuses are seeded
	t.Run("StatusSeeded", func(t *testing.T) {
		activeID, err := svc.GetFileStatusID(FileStatusActive)
		if err != nil || activeID == nil {
			t.Fatalf("Failed to get ACTIVE status: %v", err)
		}
		t.Logf("ACTIVE status ID: %d", *activeID)

		deletedID, err := svc.GetFileStatusID(FileStatusDeleted)
		if err != nil || deletedID == nil {
			t.Fatalf("Failed to get DELETED status: %v", err)
		}
		t.Logf("DELETED status ID: %d", *deletedID)

		disabledID, err := svc.GetFileStatusID(FileStatusDisabled)
		if err != nil || disabledID == nil {
			t.Fatalf("Failed to get DISABLED status: %v", err)
		}
		t.Logf("DISABLED status ID: %d", *disabledID)
	})

	// Test 2: Create file with ACTIVE status
	t.Run("CreateWithActiveStatus", func(t *testing.T) {
		activeStatusID, _ := svc.GetFileStatusID(FileStatusActive)

		entry := &FileEntry{
			ID:           "test-001",
			Name:         "test.md",
			IsDir:        false,
			Path:         "test.md",
			FileStatusID: activeStatusID,
		}

		if err := svc.CreateFileEntry(entry); err != nil {
			t.Fatalf("Failed to create entry: %v", err)
		}

		// Verify
		retrieved, _ := svc.GetFileEntryByID("test-001")
		if retrieved == nil || retrieved.FileStatusID == nil {
			t.Fatal("Could not retrieve file or status is nil")
		}

		status, _ := svc.GetFileStatusByID(*retrieved.FileStatusID)
		if *status != FileStatusActive {
			t.Fatalf("Expected ACTIVE status, got: %s", *status)
		}
		t.Logf("✓ File created with ACTIVE status")
	})

	// Test 3: Soft delete (mark as DELETED)
	t.Run("SoftDelete", func(t *testing.T) {
		if err := svc.DeleteFileEntry("test-001"); err != nil {
			t.Fatalf("Failed to delete entry: %v", err)
		}

		// Verify file still exists
		retrieved, _ := svc.GetFileEntryByID("test-001")
		if retrieved == nil {
			t.Fatal("File should still exist after soft delete")
		}

		// Verify status is DELETED
		if retrieved.FileStatusID == nil {
			t.Fatal("FileStatusID should not be nil")
		}

		status, _ := svc.GetFileStatusByID(*retrieved.FileStatusID)
		if *status != FileStatusDeleted {
			t.Fatalf("Status should be DELETED, got: %s", *status)
		}
		t.Logf("✓ File marked as DELETED")
	})

	// Test 4: Create directory with ACTIVE status
	t.Run("CreateDirectoryWithActiveStatus", func(t *testing.T) {
		activeStatusID, _ := svc.GetFileStatusID(FileStatusActive)
		dirTypeID, _ := svc.GetFileTypeID(FileTypeDirectory)

		dirEntry := &FileEntry{
			ID:           "dir-001",
			Name:         "folder",
			IsDir:        true,
			Path:         "folder",
			FileTypeID:   dirTypeID,
			FileStatusID: activeStatusID,
		}

		if err := svc.CreateFileEntry(dirEntry); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Verify
		retrieved, _ := svc.GetFileEntryByID("dir-001")
		if retrieved == nil || retrieved.FileStatusID == nil {
			t.Fatal("Could not retrieve directory or status is nil")
		}

		status, _ := svc.GetFileStatusByID(*retrieved.FileStatusID)
		if *status != FileStatusActive {
			t.Fatalf("Expected ACTIVE status, got: %s", *status)
		}
		t.Logf("✓ Directory created with ACTIVE status")
	})

	// Test 5: Soft delete directory
	t.Run("SoftDeleteDirectory", func(t *testing.T) {
		if err := svc.DeleteFileEntry("dir-001"); err != nil {
			t.Fatalf("Failed to delete directory: %v", err)
		}

		// Verify directory still exists
		retrieved, _ := svc.GetFileEntryByID("dir-001")
		if retrieved == nil {
			t.Fatal("Directory should still exist after soft delete")
		}

		// Verify status is DELETED
		status, _ := svc.GetFileStatusByID(*retrieved.FileStatusID)
		if *status != FileStatusDeleted {
			t.Fatalf("Status should be DELETED, got: %s", *status)
		}
		t.Logf("✓ Directory marked as DELETED")
	})
}
