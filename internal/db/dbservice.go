package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ServiceStatus represents the current state of the DB service
type ServiceStatus int

const (
	StatusInitializing ServiceStatus = iota
	StatusReady
	StatusStopped
	StatusError
)

func (s ServiceStatus) String() string {
	switch s {
	case StatusInitializing:
		return "initializing"
	case StatusReady:
		return "ready"
	case StatusStopped:
		return "stopped"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Note represents a persisted note record
type Note struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Path     string    `json:"path"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

// DBService manages a sqlite database and provides CRUD helpers
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

	// Optional: tune DB connection pool for single-file sqlite
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(time.Minute * 5)

	// Ping with context
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		s.setStatus(StatusError)
		return fmt.Errorf("ping sqlite: %w", err)
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
CREATE TABLE IF NOT EXISTS notes (
  id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
  title TEXT,
  path TEXT,
  created INTEGER,
  modified INTEGER
);

CREATE INDEX IF NOT EXISTS idx_notes_path ON notes(path);
`
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, schema)
	return err
}

// helper: encode string slice to JSON string
func encodeStrings(v []string) (string, error) {
	if v == nil {
		return "[]", nil
	}
	b, err := json.Marshal(v)
	return string(b), err
}

// helper: decode JSON string to string slice
func decodeStrings(in string) ([]string, error) {
	if in == "" {
		return nil, nil
	}
	var out []string
	err := json.Unmarshal([]byte(in), &out)
	return out, err
}

// CreateNote inserts a new note. `Modified` will be set if zero.
func (s *DBService) CreateNote(n *Note) error {
	db := s.getDB()
	if db == nil {
		return errors.New("db not ready")
	}
	if n == nil {
		return errors.New("nil note")
	}
	if n.Modified.IsZero() {
		n.Modified = time.Now().UTC()
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx,
		`INSERT INTO notes(title, path, created, modified) VALUES (?, ?, ?, ?)`,
		n.Title, n.Path, n.Created.Unix(), n.Modified.Unix())
	if err != nil {
		return fmt.Errorf("insert note: %w", err)
	}
	return nil
}

// GetNoteByPath retrieves a note by path.
func (s *DBService) GetNoteByPath(path string) (*Note, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	row := db.QueryRowContext(ctx, `SELECT id, title, path, created, modified FROM notes WHERE path = ?`, path)

	var (
		n         Note
		creatUnix sql.NullInt64
		modUnix   sql.NullInt64
	)
	if err := row.Scan(&n.ID, &n.Title, &n.Path, &creatUnix, &modUnix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan note: %w", err)
	}
	if creatUnix.Valid {
		n.Created = time.Unix(creatUnix.Int64, 0).UTC()
	}
	if modUnix.Valid {
		n.Modified = time.Unix(modUnix.Int64, 0).UTC()
	}
	return &n, nil
}

// GetNoteByID retrieves a note by id.
func (s *DBService) GetNoteByID(id string) (*Note, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	row := db.QueryRowContext(ctx, `SELECT id, title, path, created, modified FROM notes WHERE id = ?`, id)

	var (
		n         Note
		creatUnix sql.NullInt64
		modUnix   sql.NullInt64
	)
	if err := row.Scan(&n.ID, &n.Title, &n.Path, &creatUnix, &modUnix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan note: %w", err)
	}
	if creatUnix.Valid {
		n.Created = time.Unix(creatUnix.Int64, 0).UTC()
	}
	if modUnix.Valid {
		n.Modified = time.Unix(modUnix.Int64, 0).UTC()
	}
	return &n, nil
}

// UpdateNote updates an existing note by id.
func (s *DBService) UpdateNote(n *Note) error {
	if n == nil {
		return errors.New("nil note")
	}
	db := s.getDB()
	if db == nil {
		return errors.New("db not ready")
	}
	if n.Modified.IsZero() {
		n.Modified = time.Now().UTC()
	}

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	res, err := db.ExecContext(ctx,
		`UPDATE notes SET title = ?, path = ?, modified = ? WHERE id = ?`,
		n.Title, n.Path, n.Modified.Unix(), n.ID)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteNote removes a note by id.
func (s *DBService) DeleteNote(id string) error {
	db := s.getDB()
	if db == nil {
		return errors.New("db not ready")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	res, err := db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListNotes returns notes with optional paging.
func (s *DBService) ListNotes(limit, offset int) ([]Note, error) {
	db := s.getDB()
	if db == nil {
		return nil, errors.New("db not ready")
	}
	if limit <= 0 {
		limit = 100
	}
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `SELECT id, title, path, created, modified FROM notes ORDER BY modified DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	var out []Note
	for rows.Next() {
		var n Note
		var creatUnix sql.NullInt64
		var modUnix sql.NullInt64
		if err := rows.Scan(&n.ID, &n.Title, &n.Path, &creatUnix, &modUnix); err != nil {
			return nil, err
		}

		if creatUnix.Valid {
			n.Modified = time.Unix(creatUnix.Int64, 0).UTC()
		}
		if modUnix.Valid {
			n.Modified = time.Unix(modUnix.Int64, 0).UTC()
		}
		out = append(out, n)
	}
	return out, nil
}
