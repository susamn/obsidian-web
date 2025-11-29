package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/susamn/obsidian-web/internal/logger"
	syncpkg "github.com/susamn/obsidian-web/internal/sync"
	"github.com/susamn/obsidian-web/internal/utils"
)

// ServiceStatus represents the current state of the DB service
type ServiceStatus int

const (
	StatusInitializing ServiceStatus = iota
	StatusReady
	StatusReindexing
	StatusStopped
	StatusError
)

func (s ServiceStatus) String() string {
	switch s {
	case StatusInitializing:
		return "initializing"
	case StatusReady:
		return "ready"
	case StatusReindexing:
		return "reindexing"
	case StatusStopped:
		return "stopped"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// FileType represents a file type constant
type FileType string

const (
	FileTypeDirectory FileType = "DIRECTORY"
	FileTypeMarkdown  FileType = "MARKDOWN"
	FileTypePNG       FileType = "PNG"
	FileTypeJPEG      FileType = "JPEG"
	FileTypeJPG       FileType = "JPG"
	FileTypeGIF       FileType = "GIF"
	FileTypeWebP      FileType = "WEBP"
	FileTypeSVG       FileType = "SVG"
	FileTypePDF       FileType = "PDF"
	FileTypeTXT       FileType = "TXT"
	FileTypeJSON      FileType = "JSON"
	FileTypeYAML      FileType = "YAML"
	FileTypeXML       FileType = "XML"
	FileTypeCSV       FileType = "CSV"
	FileTypeUnknown   FileType = "UNKNOWN"
)

// FileStatus represents a file status constant
type FileStatus string

const (
	FileStatusActive   FileStatus = "ACTIVE"
	FileStatusDeleted  FileStatus = "DELETED"
	FileStatusDisabled FileStatus = "DISABLED"
)

// FileEntry represents a file or directory record
type FileEntry struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ParentID     *string   `json:"parent_id,omitempty"` // nil for root entries
	IsDir        bool      `json:"is_dir"`
	FileTypeID   *int64    `json:"file_type_id,omitempty"`   // Foreign key to file_types
	FileStatusID *int64    `json:"file_status_id,omitempty"` // Foreign key to file_statuses
	Created      time.Time `json:"created"`
	Modified     time.Time `json:"modified"`
	Size         int64     `json:"size"` // 0 for directories
	Path         string    `json:"-"`    // For internal use only, hidden from API
}

// DBService manages a sqlite database for file metadata
type DBService struct {
	ctx    context.Context
	cancel context.CancelFunc

	db     *sql.DB
	dbPath string
	dbMu   sync.RWMutex

	status   ServiceStatus
	statusMu sync.RWMutex

	wg sync.WaitGroup
}

func NewDBService(parent context.Context, dbPath *string) (*DBService, error) {
	if dbPath == nil {
		return nil, errors.New("Database path is required")
	}
	dir := filepath.Dir(*dbPath)
	if dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	ctx, cancel := context.WithCancel(parent)
	return &DBService{
		ctx:    ctx,
		cancel: cancel,
		dbPath: *dbPath,
		status: StatusInitializing,
	}, nil
}

// Start opens the sqlite database and ensures schema exists.
func (s *DBService) Start() error {
	s.statusMu.Lock()
	if s.status != StatusInitializing {
		s.statusMu.Unlock()
		return nil
	}
	s.statusMu.Unlock()

	db, err := sql.Open("sqlite3", s.dbPath+"?_foreign_keys=1")
	if err != nil {
		s.setStatus(StatusError)
		return fmt.Errorf("open sqlite: %w", err)
	}

	// Tune DB connection pool for better concurrency with workers
	// Increase from 1 to allow multiple workers to write concurrently
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Minute * 5)

	// Ping with context
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("ping sqlite: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("enable WAL mode: %w", err)
	}

	// Set synchronous mode to NORMAL for better performance (still safe with WAL)
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous=NORMAL"); err != nil {
		_ = db.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("set synchronous mode: %w", err)
	}

	// Set busy timeout to handle concurrent writes
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("set busy timeout: %w", err)
	}

	s.dbMu.Lock()
	s.db = db
	s.dbMu.Unlock()

	// Ensure schema
	if err := s.ensureSchema(); err != nil {
		_ = db.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("ensure schema: %w", err)
	}

	s.setStatus(StatusReady)
	return nil
}

// Stop closes the DB and cancels the service context.
func (s *DBService) Stop() error {
	s.statusMu.Lock()
	if s.status == StatusStopped {
		s.statusMu.Unlock()
		return nil
	}
	s.statusMu.Unlock()

	s.cancel()

	s.dbMu.Lock()
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
	s.dbMu.Unlock()

	s.setStatus(StatusStopped)
	return nil
}

func (s *DBService) GetStatus() ServiceStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.status
}

func (s *DBService) setStatus(st ServiceStatus) {
	s.statusMu.Lock()
	s.status = st
	s.statusMu.Unlock()
}

func (s *DBService) getDB() *sql.DB {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()
	return s.db
}

func (s *DBService) ensureSchema() error {
	db := s.getDB()
	if db == nil {
		return errors.New("db not initialized")
	}

	schema := `
CREATE TABLE IF NOT EXISTS file_types (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  description TEXT,
  is_binary INTEGER DEFAULT 0,
  mime_type TEXT
);

CREATE TABLE IF NOT EXISTS file_statuses (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  description TEXT
);

CREATE TABLE IF NOT EXISTS file_entries (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  parent_id TEXT,
  is_dir INTEGER NOT NULL,
  file_type_id INTEGER,
  file_status_id INTEGER,
  created INTEGER NOT NULL,
  modified INTEGER NOT NULL,
  size INTEGER,
  path TEXT NOT NULL UNIQUE,
  FOREIGN KEY (parent_id) REFERENCES file_entries(id) ON DELETE CASCADE,
  FOREIGN KEY (file_type_id) REFERENCES file_types(id),
  FOREIGN KEY (file_status_id) REFERENCES file_statuses(id)
);

CREATE INDEX IF NOT EXISTS idx_parent_id ON file_entries(parent_id);
CREATE INDEX IF NOT EXISTS idx_path ON file_entries(path);
CREATE INDEX IF NOT EXISTS idx_file_type ON file_entries(file_type_id);
CREATE INDEX IF NOT EXISTS idx_file_status ON file_entries(file_status_id);
`
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, schema); err != nil {
		return err
	}

	// Seed file types if table is empty
	if err := s.seedFileTypes(); err != nil {
		return err
	}

	// Seed file statuses if table is empty
	return s.seedFileStatuses()
}

// Global mutex for parent directory creation to prevent race conditions
// when multiple workers try to create the same parent path simultaneously
var parentDirMutex sync.Mutex

// performDatabaseUpdate updates the database for a given file event
// This is a shared helper that both vault.updateDatabase and worker.updateDatabase use
// Returns the file ID and error
// For create/modify events, returns the ID of the created/updated file
// For delete events, returns the ID of the deleted file (before deletion)
func (dbService *DBService) PerformDatabaseUpdate(vaultPath string, event syncpkg.FileChangeEvent) (string, error) {
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
		fileType := DetectFileType(filepath.Base(event.Path), isDir)
		fileTypeID, err := dbService.GetFileTypeID(fileType)
		if err != nil {
			logger.WithField("file_type", fileType).WithField("error", err).Warn("Failed to get file type ID")
		}

		// Get ACTIVE status ID
		activeStatusID, err := dbService.GetFileStatusID(FileStatusActive)
		if err != nil {
			logger.WithField("error", err).Warn("Failed to get active status ID")
		}

		// Ensure parent directories exist in the database
		var parentID *string
		parentPath := filepath.Dir(relPath)
		if parentPath != "." && parentPath != "" {
			// Item is nested, ensure parent directories exist
			parentID = dbService.EnsureParentDirsExist(vaultPath, parentPath)
		} else {
			// Item is at root level, set parent to root node ID
			rootEntry, err := dbService.GetFileEntryByPath("")
			if err == nil && rootEntry != nil {
				// Root exists, use its ID as parent
				parentID = &rootEntry.ID
			}
		}

		// Create or update file entry in database
		entry := &FileEntry{
			ID:           utils.GenerateID(),
			Name:         filepath.Base(event.Path),
			IsDir:        isDir,
			FileTypeID:   fileTypeID,
			FileStatusID: activeStatusID,
			Created:      event.Timestamp,
			Modified:     event.Timestamp,
			Size:         size,
			Path:         relPath,
			ParentID:     parentID,
		}

		// Check if entry already exists
		existing, err := dbService.GetFileEntryByPath(relPath)
		if err == nil && existing != nil {
			// Update existing entry
			entry.ID = existing.ID
			entry.Created = existing.Created
			entry.ParentID = existing.ParentID
			entry.FileTypeID = fileTypeID
			entry.FileStatusID = activeStatusID
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
		// Mark entry as deleted in database instead of removing it
		entry, err := dbService.GetFileEntryByPath(relPath)
		if err == nil && entry != nil {
			fileID := entry.ID
			if err := dbService.DeleteFileEntry(entry.ID); err != nil {
				return "", fmt.Errorf("failed to mark entry as deleted: %w", err)
			}
			return fileID, nil
		}
		return "", nil // File not found in DB, but not an error
	}

	return "", nil
}

// ensureParentDirsExist ensures all parent directories exist in the database and returns the ID of the immediate parent
func (dbService *DBService) EnsureParentDirsExist(vaultPath, parentPath string) *string {
	// Lock to prevent race conditions when multiple workers create same parent dirs
	parentDirMutex.Lock()
	defer parentDirMutex.Unlock()

	// Ensure root directory exists first
	rootEntry, err := dbService.GetFileEntryByPath("")
	var currentParentID *string
	if err != nil || rootEntry == nil {
		// Root doesn't exist, create it
		rootID := utils.GenerateID()
		dirFileTypeID, _ := dbService.GetFileTypeID(FileTypeDirectory)
		activeStatusID, _ := dbService.GetFileStatusID(FileStatusActive)
		rootEntry := &FileEntry{
			ID:           rootID,
			Name:         "vault",
			ParentID:     nil,
			IsDir:        true,
			FileTypeID:   dirFileTypeID,
			FileStatusID: activeStatusID,
			Path:         "",
			Created:      time.Now().UTC(),
			Modified:     time.Now().UTC(),
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
		dirFileTypeID, _ := dbService.GetFileTypeID(FileTypeDirectory)
		activeStatusID, _ := dbService.GetFileStatusID(FileStatusActive)
		dirEntry := &FileEntry{
			ID:           utils.GenerateID(),
			Name:         part,
			IsDir:        true,
			FileTypeID:   dirFileTypeID,
			FileStatusID: activeStatusID,
			ParentID:     currentParentID,
			Created:      time.Now().UTC(),
			Modified:     time.Now().UTC(),
			Path:         currentPath,
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

// seedFileTypes populates the file_types table with predefined types
func (s *DBService) seedFileTypes() error {
	db := s.getDB()
	if db == nil {
		return errors.New("db not initialized")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// Check if table already has data
	var count int
	row := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM file_types")
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("check file_types count: %w", err)
	}

	if count > 0 {
		return nil // Already seeded
	}

	// Define file types with metadata
	types := []struct {
		name        string
		description string
		isBinary    bool
		mimeType    string
	}{
		{string(FileTypeDirectory), "Directory", false, ""},
		{string(FileTypeMarkdown), "Markdown document", false, "text/markdown"},
		{string(FileTypePNG), "PNG image", true, "image/png"},
		{string(FileTypeJPEG), "JPEG image", true, "image/jpeg"},
		{string(FileTypeJPG), "JPG image", true, "image/jpeg"},
		{string(FileTypeGIF), "GIF image", true, "image/gif"},
		{string(FileTypeWebP), "WebP image", true, "image/webp"},
		{string(FileTypeSVG), "SVG image", false, "image/svg+xml"},
		{string(FileTypePDF), "PDF document", true, "application/pdf"},
		{string(FileTypeTXT), "Text file", false, "text/plain"},
		{string(FileTypeJSON), "JSON file", false, "application/json"},
		{string(FileTypeYAML), "YAML file", false, "application/x-yaml"},
		{string(FileTypeXML), "XML file", false, "application/xml"},
		{string(FileTypeCSV), "CSV file", false, "text/csv"},
		{string(FileTypeUnknown), "Unknown file type", true, "application/octet-stream"},
	}

	// Insert all types
	for _, ft := range types {
		_, err := db.ExecContext(ctx,
			"INSERT INTO file_types(name, description, is_binary, mime_type) VALUES (?, ?, ?, ?)",
			ft.name, ft.description, boolToInt(ft.isBinary), ft.mimeType)
		if err != nil {
			return fmt.Errorf("insert file type %s: %w", ft.name, err)
		}
	}

	return nil
}

// seedFileStatuses populates the file_statuses table with predefined statuses
func (s *DBService) seedFileStatuses() error {
	db := s.getDB()
	if db == nil {
		return errors.New("db not initialized")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// Check if table already has data
	var count int
	row := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM file_statuses")
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("check file_statuses count: %w", err)
	}

	if count > 0 {
		return nil // Already seeded
	}

	// Define file statuses with metadata
	statuses := []struct {
		name        string
		description string
	}{
		{string(FileStatusActive), "File is active and available"},
		{string(FileStatusDeleted), "File has been deleted"},
		{string(FileStatusDisabled), "File has been disabled"},
	}

	// Insert all statuses
	for _, fs := range statuses {
		_, err := db.ExecContext(ctx,
			"INSERT INTO file_statuses(name, description) VALUES (?, ?)",
			fs.name, fs.description)
		if err != nil {
			return fmt.Errorf("insert file status %s: %w", fs.name, err)
		}
	}

	return nil
}

// GetFileTypeID returns the ID for a given file type name
func (s *DBService) GetFileTypeID(fileType FileType) (*int64, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()

	var id int64
	row := db.QueryRowContext(ctx, "SELECT id FROM file_types WHERE name = ?", string(fileType))
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get file type id: %w", err)
	}

	return &id, nil
}

// GetFileTypeByID returns the file type name for a given ID
func (s *DBService) GetFileTypeByID(id int64) (*FileType, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()

	var name string
	row := db.QueryRowContext(ctx, "SELECT name FROM file_types WHERE id = ?", id)
	if err := row.Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get file type: %w", err)
	}

	ft := FileType(name)
	return &ft, nil
}

// GetFileStatusID returns the ID for a given file status name
func (s *DBService) GetFileStatusID(fileStatus FileStatus) (*int64, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()

	var id int64
	row := db.QueryRowContext(ctx, "SELECT id FROM file_statuses WHERE name = ?", string(fileStatus))
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get file status id: %w", err)
	}

	return &id, nil
}

// GetFileStatusByID returns the file status name for a given ID
func (s *DBService) GetFileStatusByID(id int64) (*FileStatus, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()

	var name string
	row := db.QueryRowContext(ctx, "SELECT name FROM file_statuses WHERE id = ?", id)
	if err := row.Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get file status: %w", err)
	}

	fs := FileStatus(name)
	return &fs, nil
}

// DetectFileType detects the file type from filename
func DetectFileType(filename string, isDir bool) FileType {
	if isDir {
		return FileTypeDirectory
	}

	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		return FileTypeMarkdown
	case ".png":
		return FileTypePNG
	case ".jpeg":
		return FileTypeJPEG
	case ".jpg":
		return FileTypeJPG
	case ".gif":
		return FileTypeGIF
	case ".webp":
		return FileTypeWebP
	case ".svg":
		return FileTypeSVG
	case ".pdf":
		return FileTypePDF
	case ".txt":
		return FileTypeTXT
	case ".json":
		return FileTypeJSON
	case ".yaml", ".yml":
		return FileTypeYAML
	case ".xml":
		return FileTypeXML
	case ".csv":
		return FileTypeCSV
	default:
		return FileTypeUnknown
	}
}

// CreateFileEntry inserts a new file or directory entry.
func (s *DBService) CreateFileEntry(entry *FileEntry) error {
	db := s.getDB()
	if db == nil {
		return errors.New("db not ready")
	}
	if entry == nil {
		return errors.New("nil entry")
	}
	if entry.Modified.IsZero() {
		entry.Modified = time.Now().UTC()
	}
	if entry.Created.IsZero() {
		entry.Created = time.Now().UTC()
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx,
		`INSERT INTO file_entries(id, name, parent_id, is_dir, file_type_id, file_status_id, created, modified, size, path)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.Name, entry.ParentID, boolToInt(entry.IsDir), entry.FileTypeID, entry.FileStatusID,
		entry.Created.Unix(), entry.Modified.Unix(), entry.Size, entry.Path)
	if err != nil {
		return fmt.Errorf("insert entry: %w", err)
	}
	return nil
}

// GetFileEntryByIDWithStatus retrieves a file entry by id with status check using JOIN
func (s *DBService) GetFileEntryByIDWithStatus(id string, status FileStatus) (*FileEntry, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()

	query := `
		SELECT fe.id, fe.name, fe.parent_id, fe.is_dir, fe.file_type_id, fe.file_status_id,
		       fe.created, fe.modified, fe.size, fe.path
		FROM file_entries fe
		INNER JOIN file_statuses fs ON fe.file_status_id = fs.id
		WHERE fe.id = ? AND fs.name = ?`

	row := db.QueryRowContext(ctx, query, id, string(status))

	var (
		entry        FileEntry
		isDirInt     int
		creatUnix    sql.NullInt64
		modUnix      sql.NullInt64
		parentID     sql.NullString
		fileTypeID   sql.NullInt64
		fileStatusID sql.NullInt64
	)
	if err := row.Scan(&entry.ID, &entry.Name, &parentID, &isDirInt, &fileTypeID, &fileStatusID,
		&creatUnix, &modUnix, &entry.Size, &entry.Path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan entry: %w", err)
	}

	if parentID.Valid {
		entry.ParentID = &parentID.String
	}
	if fileTypeID.Valid {
		entry.FileTypeID = &fileTypeID.Int64
	}
	if fileStatusID.Valid {
		entry.FileStatusID = &fileStatusID.Int64
	}
	entry.IsDir = intToBool(isDirInt)
	if creatUnix.Valid {
		entry.Created = time.Unix(creatUnix.Int64, 0).UTC()
	}
	if modUnix.Valid {
		entry.Modified = time.Unix(modUnix.Int64, 0).UTC()
	}
	return &entry, nil
}

// GetFileEntryByID retrieves a file entry by id.
func (s *DBService) GetFileEntryByID(id string) (*FileEntry, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	row := db.QueryRowContext(ctx,
		`SELECT id, name, parent_id, is_dir, file_type_id, file_status_id, created, modified, size, path FROM file_entries WHERE id = ?`, id)

	var (
		entry        FileEntry
		isDirInt     int
		creatUnix    sql.NullInt64
		modUnix      sql.NullInt64
		parentID     sql.NullString
		fileTypeID   sql.NullInt64
		fileStatusID sql.NullInt64
	)
	if err := row.Scan(&entry.ID, &entry.Name, &parentID, &isDirInt, &fileTypeID, &fileStatusID,
		&creatUnix, &modUnix, &entry.Size, &entry.Path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan entry: %w", err)
	}

	if parentID.Valid {
		entry.ParentID = &parentID.String
	}
	if fileTypeID.Valid {
		entry.FileTypeID = &fileTypeID.Int64
	}
	if fileStatusID.Valid {
		entry.FileStatusID = &fileStatusID.Int64
	}
	entry.IsDir = intToBool(isDirInt)
	if creatUnix.Valid {
		entry.Created = time.Unix(creatUnix.Int64, 0).UTC()
	}
	if modUnix.Valid {
		entry.Modified = time.Unix(modUnix.Int64, 0).UTC()
	}
	return &entry, nil
}

// GetFileEntryByPathWithStatus retrieves a file entry by path with status check using JOIN
func (s *DBService) GetFileEntryByPathWithStatus(path string, status FileStatus) (*FileEntry, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()

	query := `
		SELECT fe.id, fe.name, fe.parent_id, fe.is_dir, fe.file_type_id, fe.file_status_id,
		       fe.created, fe.modified, fe.size, fe.path
		FROM file_entries fe
		INNER JOIN file_statuses fs ON fe.file_status_id = fs.id
		WHERE fe.path = ? AND fs.name = ?`

	row := db.QueryRowContext(ctx, query, path, string(status))

	var (
		entry        FileEntry
		isDirInt     int
		creatUnix    sql.NullInt64
		modUnix      sql.NullInt64
		parentID     sql.NullString
		fileTypeID   sql.NullInt64
		fileStatusID sql.NullInt64
	)
	if err := row.Scan(&entry.ID, &entry.Name, &parentID, &isDirInt, &fileTypeID, &fileStatusID,
		&creatUnix, &modUnix, &entry.Size, &entry.Path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan entry: %w", err)
	}

	if parentID.Valid {
		entry.ParentID = &parentID.String
	}
	if fileTypeID.Valid {
		entry.FileTypeID = &fileTypeID.Int64
	}
	if fileStatusID.Valid {
		entry.FileStatusID = &fileStatusID.Int64
	}
	entry.IsDir = intToBool(isDirInt)
	if creatUnix.Valid {
		entry.Created = time.Unix(creatUnix.Int64, 0).UTC()
	}
	if modUnix.Valid {
		entry.Modified = time.Unix(modUnix.Int64, 0).UTC()
	}
	return &entry, nil
}

// GetFileEntryByPath retrieves a file entry by path.
func (s *DBService) GetFileEntryByPath(path string) (*FileEntry, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	row := db.QueryRowContext(ctx,
		`SELECT id, name, parent_id, is_dir, file_type_id, file_status_id, created, modified, size, path FROM file_entries WHERE path = ?`, path)

	var (
		entry        FileEntry
		isDirInt     int
		creatUnix    sql.NullInt64
		modUnix      sql.NullInt64
		parentID     sql.NullString
		fileTypeID   sql.NullInt64
		fileStatusID sql.NullInt64
	)
	if err := row.Scan(&entry.ID, &entry.Name, &parentID, &isDirInt, &fileTypeID, &fileStatusID,
		&creatUnix, &modUnix, &entry.Size, &entry.Path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan entry: %w", err)
	}

	if parentID.Valid {
		entry.ParentID = &parentID.String
	}
	if fileTypeID.Valid {
		entry.FileTypeID = &fileTypeID.Int64
	}
	if fileStatusID.Valid {
		entry.FileStatusID = &fileStatusID.Int64
	}
	entry.IsDir = intToBool(isDirInt)
	if creatUnix.Valid {
		entry.Created = time.Unix(creatUnix.Int64, 0).UTC()
	}
	if modUnix.Valid {
		entry.Modified = time.Unix(modUnix.Int64, 0).UTC()
	}
	return &entry, nil
}

// GetFilePathByID retrieves the path of a file entry by its ID.
func (s *DBService) GetFilePathByID(id string) (string, error) {
	entry, err := s.GetFileEntryByID(id)
	if err != nil {
		return "", err
	}
	if entry == nil {
		return "", errors.New("file entry not found")
	}
	return entry.Path, nil
}

// GetFileEntriesByParentID retrieves all entries in a directory.
func (s *DBService) GetFileEntriesByParentID(parentID *string) ([]FileEntry, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	var query string
	var args []interface{}
	if parentID == nil {
		query = `
			SELECT fe.id, fe.name, fe.parent_id, fe.is_dir, fe.file_type_id, fe.file_status_id,
			       fe.created, fe.modified, fe.size, fe.path
			FROM file_entries fe
			INNER JOIN file_statuses fs ON fe.file_status_id = fs.id
			WHERE fe.parent_id IS NULL AND fs.name = ?
			ORDER BY fe.is_dir DESC, fe.name ASC`
		args = []interface{}{string(FileStatusActive)}
	} else {
		query = `
			SELECT fe.id, fe.name, fe.parent_id, fe.is_dir, fe.file_type_id, fe.file_status_id,
			       fe.created, fe.modified, fe.size, fe.path
			FROM file_entries fe
			INNER JOIN file_statuses fs ON fe.file_status_id = fs.id
			WHERE fe.parent_id = ? AND fs.name = ?
			ORDER BY fe.is_dir DESC, fe.name ASC`
		args = []interface{}{*parentID, string(FileStatusActive)}
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query entries: %w", err)
	}
	defer rows.Close()

	var out []FileEntry
	for rows.Next() {
		var entry FileEntry
		var isDirInt int
		var creatUnix sql.NullInt64
		var modUnix sql.NullInt64
		var parentIDStr sql.NullString
		var fileTypeID sql.NullInt64
		var fileStatusID sql.NullInt64

		if err := rows.Scan(&entry.ID, &entry.Name, &parentIDStr, &isDirInt, &fileTypeID, &fileStatusID,
			&creatUnix, &modUnix, &entry.Size, &entry.Path); err != nil {
			return nil, err
		}

		if parentIDStr.Valid {
			entry.ParentID = &parentIDStr.String
		}
		if fileTypeID.Valid {
			entry.FileTypeID = &fileTypeID.Int64
		}
		if fileStatusID.Valid {
			entry.FileStatusID = &fileStatusID.Int64
		}
		entry.IsDir = intToBool(isDirInt)
		if creatUnix.Valid {
			entry.Created = time.Unix(creatUnix.Int64, 0).UTC()
		}
		if modUnix.Valid {
			entry.Modified = time.Unix(modUnix.Int64, 0).UTC()
		}
		out = append(out, entry)
	}
	return out, nil
}

// UpdateFileEntry updates an existing entry by id.
func (s *DBService) UpdateFileEntry(entry *FileEntry) error {
	if entry == nil {
		return errors.New("nil entry")
	}
	db := s.getDB()
	if db == nil {
		return errors.New("db not ready")
	}
	if entry.Modified.IsZero() {
		entry.Modified = time.Now().UTC()
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	res, err := db.ExecContext(ctx,
		`UPDATE file_entries SET name = ?, parent_id = ?, file_type_id = ?, file_status_id = ?, modified = ?, size = ?, path = ? WHERE id = ?`,
		entry.Name, entry.ParentID, entry.FileTypeID, entry.FileStatusID, entry.Modified.Unix(), entry.Size, entry.Path, entry.ID)
	if err != nil {
		return fmt.Errorf("update entry: %w", err)
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteFileEntry marks an entry as deleted by setting its status to DELETED instead of removing it.
func (s *DBService) DeleteFileEntry(id string) error {
	db := s.getDB()
	if db == nil {
		return errors.New("db not ready")
	}

	// Get the DELETED status ID
	deletedStatusID, err := s.GetFileStatusID(FileStatusDeleted)
	if err != nil {
		return fmt.Errorf("failed to get deleted status ID: %w", err)
	}
	if deletedStatusID == nil {
		return errors.New("deleted status not found in database")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	res, err := db.ExecContext(ctx, `UPDATE file_entries SET file_status_id = ?, modified = ? WHERE id = ?`,
		*deletedStatusID, time.Now().UTC().Unix(), id)
	if err != nil {
		return fmt.Errorf("mark entry as deleted: %w", err)
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ClearAll deletes all entries from the database.
func (s *DBService) ClearAll() error {
	db := s.getDB()
	if db == nil {
		return errors.New("db not ready")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `DELETE FROM file_entries`)
	if err != nil {
		return fmt.Errorf("clear all: %w", err)
	}
	return nil
}

// DisableAllFiles sets the status of all files to DISABLED.
// This is used during full reindexing to invalidate all existing entries.
func (s *DBService) DisableAllFiles() error {
	db := s.getDB()
	if db == nil {
		return errors.New("db not ready")
	}

	// Get the DISABLED status ID
	disabledStatusID, err := s.GetFileStatusID(FileStatusDisabled)
	if err != nil {
		return fmt.Errorf("failed to get disabled status ID: %w", err)
	}
	if disabledStatusID == nil {
		return errors.New("disabled status not found in database")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()

	// Update all entries to DISABLED
	_, err = db.ExecContext(ctx, `UPDATE file_entries SET file_status_id = ?`, *disabledStatusID)
	if err != nil {
		return fmt.Errorf("disable all files: %w", err)
	}

	return nil
}

// GetFileEntryByName finds a file entry by name (for resolving wikilinks)
// This searches for an exact match or match with .md extension
func (s *DBService) GetFileEntryByName(name string) (*FileEntry, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}

	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()

	// Try exact match first
	var entry FileEntry
	var isDirInt int
	var creatUnix sql.NullInt64
	var modUnix sql.NullInt64
	var parentID sql.NullString
	var fileTypeID sql.NullInt64
	var fileStatusID sql.NullInt64

	row := db.QueryRowContext(ctx,
		`SELECT id, name, parent_id, is_dir, file_type_id, file_status_id, created, modified, size, path
		 FROM file_entries
		 WHERE name = ? OR name = ?
		 LIMIT 1`,
		name, name+".md")

	if err := row.Scan(&entry.ID, &entry.Name, &parentID, &isDirInt, &fileTypeID, &fileStatusID,
		&creatUnix, &modUnix, &entry.Size, &entry.Path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan entry: %w", err)
	}

	if parentID.Valid {
		entry.ParentID = &parentID.String
	}
	if fileTypeID.Valid {
		entry.FileTypeID = &fileTypeID.Int64
	}
	if fileStatusID.Valid {
		entry.FileStatusID = &fileStatusID.Int64
	}
	entry.IsDir = intToBool(isDirInt)
	if creatUnix.Valid {
		entry.Created = time.Unix(creatUnix.Int64, 0).UTC()
	}
	if modUnix.Valid {
		entry.Modified = time.Unix(modUnix.Int64, 0).UTC()
	}

	return &entry, nil
}

// Helper functions
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}
