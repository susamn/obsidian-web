package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewDBService tests database service creation
func TestNewDBService(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	svc, err := NewDBService(context.Background(), &dbPath)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}

	if svc == nil {
		t.Fatal("DBService is nil")
	}

	if svc.dbPath != dbPath {
		t.Errorf("Expected dbPath %s, got %s", dbPath, svc.dbPath)
	}

	if svc.GetStatus() != StatusInitializing {
		t.Errorf("Expected status %s, got %s", StatusInitializing, svc.GetStatus())
	}
}

// TestNewDBService_NilPath tests error handling for nil path
func TestNewDBService_NilPath(t *testing.T) {
	_, err := NewDBService(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error for nil path, got nil")
	}
}

// TestStart tests database service startup
func TestStart(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	svc, err := NewDBService(context.Background(), &dbPath)
	if err != nil {
		t.Fatalf("Failed to create DBService: %v", err)
	}

	if err := svc.Start(); err != nil {
		t.Fatalf("Failed to start DBService: %v", err)
	}

	if svc.GetStatus() != StatusReady {
		t.Errorf("Expected status %s, got %s", StatusReady, svc.GetStatus())
	}

	// Verify database file was created
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("Database file not created: %v", err)
	}

	svc.Stop()
}

// TestCreateFileEntry tests creating file entries
func TestCreateFileEntry(t *testing.T) {
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

	entry := &FileEntry{
		ID:       "file-001",
		Name:     "test.md",
		ParentID: nil,
		IsDir:    false,
		Created:  time.Now().UTC(),
		Modified: time.Now().UTC(),
		Size:     100,
		Path:     "test.md",
	}

	if err := svc.CreateFileEntry(entry); err != nil {
		t.Fatalf("Failed to create file entry: %v", err)
	}

	// Verify entry was created
	retrieved, err := svc.GetFileEntryByID("file-001")
	if err != nil {
		t.Fatalf("Failed to retrieve entry: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved entry is nil")
	}

	if retrieved.Name != "test.md" {
		t.Errorf("Expected name 'test.md', got '%s'", retrieved.Name)
	}

	if retrieved.Size != 100 {
		t.Errorf("Expected size 100, got %d", retrieved.Size)
	}
}

// TestGetFileEntryByID tests retrieving entries by ID
func TestGetFileEntryByID(t *testing.T) {
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

	entry := &FileEntry{
		ID:       "file-001",
		Name:     "test.md",
		ParentID: nil,
		IsDir:    false,
		Path:     "test.md",
	}

	svc.CreateFileEntry(entry)

	// Test retrieval
	retrieved, err := svc.GetFileEntryByID("file-001")
	if err != nil {
		t.Fatalf("Failed to get entry: %v", err)
	}

	if retrieved.ID != "file-001" {
		t.Errorf("Expected ID 'file-001', got '%s'", retrieved.ID)
	}

	// Test non-existent entry
	retrieved, err = svc.GetFileEntryByID("nonexistent")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if retrieved != nil {
		t.Fatal("Expected nil for non-existent entry")
	}
}

// TestGetFileEntryByPath tests retrieving entries by path
func TestGetFileEntryByPath(t *testing.T) {
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

	entry := &FileEntry{
		ID:       "file-001",
		Name:     "test.md",
		ParentID: nil,
		IsDir:    false,
		Path:     "test.md",
	}

	svc.CreateFileEntry(entry)

	// Test retrieval by path
	retrieved, err := svc.GetFileEntryByPath("test.md")
	if err != nil {
		t.Fatalf("Failed to get entry: %v", err)
	}

	if retrieved.ID != "file-001" {
		t.Errorf("Expected ID 'file-001', got '%s'", retrieved.ID)
	}

	// Test non-existent path
	retrieved, err = svc.GetFileEntryByPath("nonexistent.md")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if retrieved != nil {
		t.Fatal("Expected nil for non-existent path")
	}
}

// TestGetFileEntriesByParentID tests retrieving entries by parent ID
func TestGetFileEntriesByParentID(t *testing.T) {
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

	// Get ACTIVE status ID
	activeStatusID, _ := svc.GetFileStatusID(FileStatusActive)

	// Create parent directory
	parentEntry := &FileEntry{
		ID:           "dir-001",
		Name:         "folder",
		ParentID:     nil,
		IsDir:        true,
		Path:         "folder",
		FileStatusID: activeStatusID,
	}
	svc.CreateFileEntry(parentEntry)

	// Create child files
	child1 := &FileEntry{
		ID:           "file-001",
		Name:         "file1.md",
		ParentID:     &parentEntry.ID,
		IsDir:        false,
		Path:         "folder/file1.md",
		FileStatusID: activeStatusID,
	}

	child2 := &FileEntry{
		ID:           "file-002",
		Name:         "file2.md",
		ParentID:     &parentEntry.ID,
		IsDir:        false,
		Path:         "folder/file2.md",
		FileStatusID: activeStatusID,
	}

	svc.CreateFileEntry(child1)
	svc.CreateFileEntry(child2)

	// Retrieve children
	children, err := svc.GetFileEntriesByParentID(&parentEntry.ID)
	if err != nil {
		t.Fatalf("Failed to get children: %v", err)
	}

	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}

	// Test root entries
	rootEntries, err := svc.GetFileEntriesByParentID(nil)
	if err != nil {
		t.Fatalf("Failed to get root entries: %v", err)
	}

	if len(rootEntries) < 1 {
		t.Errorf("Expected at least 1 root entry, got %d", len(rootEntries))
	}
}

// TestUpdateFileEntry tests updating entries
func TestUpdateFileEntry(t *testing.T) {
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

	// Get ACTIVE status ID
	activeStatusID, _ := svc.GetFileStatusID(FileStatusActive)

	// Create entry
	entry := &FileEntry{
		ID:           "file-001",
		Name:         "test.md",
		ParentID:     nil,
		IsDir:        false,
		Path:         "test.md",
		Size:         100,
		FileStatusID: activeStatusID,
	}
	svc.CreateFileEntry(entry)

	// Update entry
	entry.Name = "renamed.md"
	entry.Path = "renamed.md"
	entry.Size = 200

	if err := svc.UpdateFileEntry(entry); err != nil {
		t.Fatalf("Failed to update entry: %v", err)
	}

	// Verify update
	retrieved, err := svc.GetFileEntryByID("file-001")
	if err != nil {
		t.Fatalf("Failed to retrieve updated entry: %v", err)
	}

	if retrieved.Name != "renamed.md" {
		t.Errorf("Expected name 'renamed.md', got '%s'", retrieved.Name)
	}

	if retrieved.Size != 200 {
		t.Errorf("Expected size 200, got %d", retrieved.Size)
	}
}

// TestDeleteFileEntry tests deleting entries
func TestDeleteFileEntry(t *testing.T) {
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

	// Create entry
	entry := &FileEntry{
		ID:       "file-001",
		Name:     "test.md",
		ParentID: nil,
		IsDir:    false,
		Path:     "test.md",
	}
	svc.CreateFileEntry(entry)

	// Delete entry (marks as DELETED)
	if err := svc.DeleteFileEntry("file-001"); err != nil {
		t.Fatalf("Failed to delete entry: %v", err)
	}

	// Verify entry is marked as deleted (not removed)
	retrieved, err := svc.GetFileEntryByID("file-001")
	if err != nil {
		t.Fatalf("Failed to check deletion: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Entry should still exist after soft delete")
	}

	// Check that status is DELETED
	if retrieved.FileStatusID == nil {
		t.Fatal("FileStatusID should not be nil")
	}

	deletedStatus, err := svc.GetFileStatusByID(*retrieved.FileStatusID)
	if err != nil {
		t.Fatalf("Failed to get file status: %v", err)
	}

	if deletedStatus == nil || *deletedStatus != FileStatusDeleted {
		t.Fatalf("Entry should have DELETED status, got: %v", deletedStatus)
	}
}

// TestCascadeDelete tests cascade delete with foreign keys
func TestCascadeDelete(t *testing.T) {
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

	// Create parent directory
	parentEntry := &FileEntry{
		ID:       "dir-001",
		Name:     "folder",
		ParentID: nil,
		IsDir:    true,
		Path:     "folder",
	}
	svc.CreateFileEntry(parentEntry)

	// Create child file
	child := &FileEntry{
		ID:       "file-001",
		Name:     "file1.md",
		ParentID: &parentEntry.ID,
		IsDir:    false,
		Path:     "folder/file1.md",
	}
	svc.CreateFileEntry(child)

	// Delete parent (marks as DELETED, no cascade)
	if err := svc.DeleteFileEntry(parentEntry.ID); err != nil {
		t.Fatalf("Failed to delete parent: %v", err)
	}

	// Verify parent is marked as deleted (not removed)
	retrieved, err := svc.GetFileEntryByID(parentEntry.ID)
	if err != nil {
		t.Fatalf("Failed to check parent deletion: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Parent should still exist after soft delete")
	}

	// Check that parent status is DELETED
	if retrieved.FileStatusID == nil {
		t.Fatal("Parent FileStatusID should not be nil")
	}
	deletedStatus, err := svc.GetFileStatusByID(*retrieved.FileStatusID)
	if err != nil {
		t.Fatalf("Failed to get parent file status: %v", err)
	}
	if deletedStatus == nil || *deletedStatus != FileStatusDeleted {
		t.Fatalf("Parent should have DELETED status, got: %v", deletedStatus)
	}

	// Verify child is NOT cascade deleted (soft delete doesn't cascade)
	childRetrieved, err := svc.GetFileEntryByID("file-001")
	if err != nil {
		t.Fatalf("Failed to check child: %v", err)
	}
	if childRetrieved == nil {
		t.Fatal("Child should still exist (soft delete doesn't cascade)")
	}
	// Child should still be ACTIVE (or whatever status it had)
}

// TestClearAll tests clearing all entries
func TestClearAll(t *testing.T) {
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

	// Create multiple entries
	for i := 0; i < 5; i++ {
		entry := &FileEntry{
			ID:   "file-00" + string(rune(48+i)),
			Name: "file" + string(rune(48+i)) + ".md",
			Path: "file" + string(rune(48+i)) + ".md",
		}
		svc.CreateFileEntry(entry)
	}

	// Clear all
	if err := svc.ClearAll(); err != nil {
		t.Fatalf("Failed to clear all: %v", err)
	}

	// Verify all entries are deleted
	entries, err := svc.GetFileEntriesByParentID(nil)
	if err != nil {
		t.Fatalf("Failed to get root entries: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(entries))
	}
}

// TestFileEntryTimestamps tests timestamp handling
func TestFileEntryTimestamps(t *testing.T) {
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

	now := time.Now().UTC()
	entry := &FileEntry{
		ID:       "file-001",
		Name:     "test.md",
		ParentID: nil,
		IsDir:    false,
		Path:     "test.md",
		Created:  now,
		Modified: now,
	}

	svc.CreateFileEntry(entry)

	retrieved, err := svc.GetFileEntryByID("file-001")
	if err != nil {
		t.Fatalf("Failed to retrieve entry: %v", err)
	}

	// Verify timestamps are preserved (allow 1 second difference for rounding)
	if retrieved.Created.Unix() != now.Unix() {
		t.Errorf("Expected created time %v, got %v", now.Unix(), retrieved.Created.Unix())
	}

	if retrieved.Modified.Unix() != now.Unix() {
		t.Errorf("Expected modified time %v, got %v", now.Unix(), retrieved.Modified.Unix())
	}
}

// TestParentIDPointer tests proper handling of nil and non-nil parent IDs
func TestParentIDPointer(t *testing.T) {
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

	// Create root entry with nil parent
	rootEntry := &FileEntry{
		ID:       "root-001",
		Name:     "root",
		ParentID: nil,
		IsDir:    true,
		Path:     "root",
	}

	svc.CreateFileEntry(rootEntry)

	retrieved, _ := svc.GetFileEntryByID("root-001")
	if retrieved.ParentID != nil {
		t.Fatal("Expected nil ParentID for root entry")
	}

	// Create child with parent
	childEntry := &FileEntry{
		ID:       "child-001",
		Name:     "child",
		ParentID: &rootEntry.ID,
		IsDir:    true,
		Path:     "root/child",
	}

	svc.CreateFileEntry(childEntry)

	childRetrieved, _ := svc.GetFileEntryByID("child-001")
	if childRetrieved.ParentID == nil {
		t.Fatal("Expected non-nil ParentID for child entry")
	}

	if *childRetrieved.ParentID != rootEntry.ID {
		t.Errorf("Expected ParentID %s, got %s", rootEntry.ID, *childRetrieved.ParentID)
	}
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		isDir    bool
		expected FileType
	}{
		{"Directory", "folder", true, FileTypeDirectory},
		{"Markdown", "note.md", false, FileTypeMarkdown},
		{"Markdown alt", "note.markdown", false, FileTypeMarkdown},
		{"PNG", "image.png", false, FileTypePNG},
		{"JPEG", "image.jpg", false, FileTypeJPG},
		{"Text", "file.txt", false, FileTypeTXT},
		{"Unknown", "file.unknown", false, FileTypeUnknown},
		{"No extension", "file", false, FileTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectFileType(tt.filename, tt.isDir)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetFileTypeID(t *testing.T) {
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

	// Test getting ID for known type
	id, err := svc.GetFileTypeID(FileTypeMarkdown)
	if err != nil {
		t.Fatalf("Failed to get file type ID: %v", err)
	}
	if id == nil {
		t.Fatal("Expected non-nil ID for Markdown")
	}

	// Test getting ID for another known type
	id2, err := svc.GetFileTypeID(FileTypeDirectory)
	if err != nil {
		t.Fatalf("Failed to get file type ID: %v", err)
	}
	if id2 == nil {
		t.Fatal("Expected non-nil ID for Directory")
	}
	if *id == *id2 {
		t.Error("Expected different IDs for different types")
	}

	// Test getting ID for non-existent type (should return nil, nil or error depending on impl)
	// Based on code, it queries by name string. If seed works, it should be fine.
	// If I ask for a type that wasn't seeded...
	id3, err := svc.GetFileTypeID("NON_EXISTENT_TYPE")
	if err != nil {
		// It might return error or nil if not found (code returns nil, nil on sql.ErrNoRows)
	}
	if id3 != nil {
		t.Error("Expected nil ID for non-existent type")
	}
}

func TestGetFileTypeByID(t *testing.T) {
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

	// Get ID first
	id, err := svc.GetFileTypeID(FileTypeMarkdown)
	if err != nil || id == nil {
		t.Fatalf("Failed to get markdown ID: %v", err)
	}

	// Get Type by ID
	ft, err := svc.GetFileTypeByID(*id)
	if err != nil {
		t.Fatalf("Failed to get file type by ID: %v", err)
	}
	if ft == nil {
		t.Fatal("Expected non-nil file type")
	}
	if *ft != FileTypeMarkdown {
		t.Errorf("Expected %s, got %s", FileTypeMarkdown, *ft)
	}

	// Test non-existent ID
	ft2, err := svc.GetFileTypeByID(999999)
	if err != nil {
		// nil, nil on ErrNoRows
	}
	if ft2 != nil {
		t.Error("Expected nil for non-existent ID")
	}
}
