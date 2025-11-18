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

	// Create parent directory
	parentEntry := &FileEntry{
		ID:       "dir-001",
		Name:     "folder",
		ParentID: nil,
		IsDir:    true,
		Path:     "folder",
	}
	svc.CreateFileEntry(parentEntry)

	// Create child files
	child1 := &FileEntry{
		ID:       "file-001",
		Name:     "file1.md",
		ParentID: &parentEntry.ID,
		IsDir:    false,
		Path:     "folder/file1.md",
	}

	child2 := &FileEntry{
		ID:       "file-002",
		Name:     "file2.md",
		ParentID: &parentEntry.ID,
		IsDir:    false,
		Path:     "folder/file2.md",
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

	// Create entry
	entry := &FileEntry{
		ID:       "file-001",
		Name:     "test.md",
		ParentID: nil,
		IsDir:    false,
		Path:     "test.md",
		Size:     100,
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

	// Delete entry
	if err := svc.DeleteFileEntry("file-001"); err != nil {
		t.Fatalf("Failed to delete entry: %v", err)
	}

	// Verify deletion
	retrieved, err := svc.GetFileEntryByID("file-001")
	if err != nil {
		t.Fatalf("Failed to check deletion: %v", err)
	}

	if retrieved != nil {
		t.Fatal("Entry should be deleted but was found")
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

	// Delete parent (should cascade)
	if err := svc.DeleteFileEntry(parentEntry.ID); err != nil {
		t.Fatalf("Failed to delete parent: %v", err)
	}

	// Verify parent is deleted
	retrieved, _ := svc.GetFileEntryByID(parentEntry.ID)
	if retrieved != nil {
		t.Fatal("Parent should be deleted")
	}

	// Verify child is also deleted (cascade)
	childRetrieved, _ := svc.GetFileEntryByID("file-001")
	if childRetrieved != nil {
		t.Fatal("Child should be cascade deleted")
	}
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
