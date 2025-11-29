// Package vecdb provides an embeddable vector database for Go applications.
// It supports disk persistence, efficient similarity search using HNSW algorithm,
// and provides APIs for training (inserting), deleting, and querying vectors.
package vector

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrNotFound          = errors.New("vector not found")
	ErrDimensionMismatch = errors.New("dimension mismatch")
	ErrEmptyVector       = errors.New("empty vector")
	ErrDatabaseClosed    = errors.New("database is closed")
	ErrInvalidK          = errors.New("k must be positive")
)

// DistanceFunc defines the signature for distance calculation functions.
type DistanceFunc func(a, b []float32) float32

// Config holds the configuration for the vector database.
type Config struct {
	// Dimension is the size of vectors stored in the database.
	Dimension int
	// StoragePath is the directory where data will be persisted.
	StoragePath string
	// Distance is the distance function to use (default: Cosine).
	Distance DistanceFunc
	// M is the max number of connections per layer in HNSW (default: 16).
	M int
	// EfConstruction is the size of dynamic candidate list during construction (default: 200).
	EfConstruction int
	// EfSearch is the size of dynamic candidate list during search (default: 50).
	EfSearch int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(dimension int, storagePath string) Config {
	return Config{
		Dimension:      dimension,
		StoragePath:    storagePath,
		Distance:       CosineDistance,
		M:              16,
		EfConstruction: 200,
		EfSearch:       50,
	}
}

// Vector represents a stored vector with its ID and optional metadata.
type Vector struct {
	ID       string
	Data     []float32
	Metadata map[string]string
}

// SearchResult represents a single search result.
type SearchResult struct {
	ID       string
	Distance float32
	Metadata map[string]string
}

// DB is the main vector database struct.
type DB struct {
	mu     sync.RWMutex
	config Config
	index  *hnswIndex
	store  *storage
	closed bool
}

// Open creates or opens a vector database at the specified path.
func Open(config Config) (*DB, error) {
	if config.Dimension <= 0 {
		return nil, errors.New("dimension must be positive")
	}
	if config.StoragePath == "" {
		return nil, errors.New("storage path is required")
	}
	if config.Distance == nil {
		config.Distance = CosineDistance
	}
	if config.M <= 0 {
		config.M = 16
	}
	if config.EfConstruction <= 0 {
		config.EfConstruction = 200
	}
	if config.EfSearch <= 0 {
		config.EfSearch = 50
	}

	if err := os.MkdirAll(config.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	store, err := openStorage(config.StoragePath, config.Dimension)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage: %w", err)
	}

	index := newHNSWIndex(config.Dimension, config.M, config.EfConstruction, config.Distance)

	db := &DB{
		config: config,
		index:  index,
		store:  store,
	}

	// Load existing data into the index
	if err := db.loadFromStorage(); err != nil {
		store.close()
		return nil, fmt.Errorf("failed to load existing data: %w", err)
	}

	return db, nil
}

// loadFromStorage loads all vectors from disk into the HNSW index.
func (db *DB) loadFromStorage() error {
	vectors, err := db.store.loadAll()
	if err != nil {
		return err
	}

	for _, v := range vectors {
		db.index.insert(v.ID, v.Data)
	}

	return nil
}

// Train adds or updates a vector in the database.
// This is the main API for adding new data to the database.
func (db *DB) Train(id string, vector []float32, metadata map[string]string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}
	if len(vector) == 0 {
		return ErrEmptyVector
	}
	if len(vector) != db.config.Dimension {
		return fmt.Errorf("%w: expected %d, got %d", ErrDimensionMismatch, db.config.Dimension, len(vector))
	}

	v := &Vector{
		ID:       id,
		Data:     vector,
		Metadata: metadata,
	}

	// Persist to disk first
	if err := db.store.save(v); err != nil {
		return fmt.Errorf("failed to persist vector: %w", err)
	}

	// Update or insert into index
	if db.index.has(id) {
		db.index.delete(id)
	}
	db.index.insert(id, vector)

	return nil
}

// TrainBatch adds multiple vectors to the database efficiently.
func (db *DB) TrainBatch(vectors []*Vector) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	for _, v := range vectors {
		if len(v.Data) != db.config.Dimension {
			return fmt.Errorf("%w for id %s: expected %d, got %d", ErrDimensionMismatch, v.ID, db.config.Dimension, len(v.Data))
		}
	}

	// Persist all vectors
	if err := db.store.saveBatch(vectors); err != nil {
		return fmt.Errorf("failed to persist vectors: %w", err)
	}

	// Update index
	for _, v := range vectors {
		if db.index.has(v.ID) {
			db.index.delete(v.ID)
		}
		db.index.insert(v.ID, v.Data)
	}

	return nil
}

// Delete removes a vector from the database by ID.
func (db *DB) Delete(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if !db.index.has(id) {
		return ErrNotFound
	}

	// Remove from disk
	if err := db.store.delete(id); err != nil {
		return fmt.Errorf("failed to delete from storage: %w", err)
	}

	// Remove from index
	db.index.delete(id)

	return nil
}

// DeleteBatch removes multiple vectors from the database.
func (db *DB) DeleteBatch(ids []string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	for _, id := range ids {
		if !db.index.has(id) {
			continue // Skip non-existent IDs
		}
		if err := db.store.delete(id); err != nil {
			return fmt.Errorf("failed to delete %s from storage: %w", id, err)
		}
		db.index.delete(id)
	}

	return nil
}

// Query performs a similarity search and returns the k nearest neighbors.
func (db *DB) Query(vector []float32, k int) ([]SearchResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, ErrDatabaseClosed
	}
	if len(vector) != db.config.Dimension {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrDimensionMismatch, db.config.Dimension, len(vector))
	}
	if k <= 0 {
		return nil, ErrInvalidK
	}

	// Search in the HNSW index
	neighbors := db.index.search(vector, k, db.config.EfSearch)

	results := make([]SearchResult, 0, len(neighbors))
	for _, n := range neighbors {
		v, err := db.store.get(n.id)
		if err != nil {
			continue // Skip if we can't load metadata
		}
		results = append(results, SearchResult{
			ID:       n.id,
			Distance: n.distance,
			Metadata: v.Metadata,
		})
	}

	return results, nil
}

// QueryWithFilter performs a similarity search with a metadata filter.
func (db *DB) QueryWithFilter(vector []float32, k int, filter func(map[string]string) bool) ([]SearchResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, ErrDatabaseClosed
	}
	if len(vector) != db.config.Dimension {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrDimensionMismatch, db.config.Dimension, len(vector))
	}
	if k <= 0 {
		return nil, ErrInvalidK
	}

	// Search more candidates to account for filtering
	searchK := k * 10
	if searchK > db.index.size() {
		searchK = db.index.size()
	}
	if searchK < k {
		searchK = k
	}

	neighbors := db.index.search(vector, searchK, db.config.EfSearch)

	results := make([]SearchResult, 0, k)
	for _, n := range neighbors {
		if len(results) >= k {
			break
		}
		v, err := db.store.get(n.id)
		if err != nil {
			continue
		}
		if filter != nil && !filter(v.Metadata) {
			continue
		}
		results = append(results, SearchResult{
			ID:       n.id,
			Distance: n.distance,
			Metadata: v.Metadata,
		})
	}

	return results, nil
}

// Get retrieves a specific vector by ID.
func (db *DB) Get(id string) (*Vector, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, ErrDatabaseClosed
	}

	return db.store.get(id)
}

// Has checks if a vector with the given ID exists.
func (db *DB) Has(id string) bool {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return false
	}

	return db.index.has(id)
}

// Count returns the number of vectors in the database.
func (db *DB) Count() int {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return db.index.size()
}

// Close closes the database and flushes all data to disk.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	db.closed = true
	return db.store.close()
}

// Compact rewrites the storage to reclaim space from deleted vectors.
func (db *DB) Compact() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	return db.store.compact()
}

// --- Distance Functions ---

// CosineDistance calculates 1 - cosine similarity between two vectors.
func CosineDistance(a, b []float32) float32 {
	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	similarity := dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
	return 1.0 - similarity
}

// EuclideanDistance calculates the L2 distance between two vectors.
func EuclideanDistance(a, b []float32) float32 {
	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return float32(math.Sqrt(float64(sum)))
}

// DotProductDistance calculates negative dot product (for max inner product search).
func DotProductDistance(a, b []float32) float32 {
	var dotProduct float32
	for i := range a {
		dotProduct += a[i] * b[i]
	}
	return -dotProduct
}

// ManhattanDistance calculates the L1 distance between two vectors.
func ManhattanDistance(a, b []float32) float32 {
	var sum float32
	for i := range a {
		diff := a[i] - b[i]
		if diff < 0 {
			diff = -diff
		}
		sum += diff
	}
	return sum
}

// --- Binary encoding helpers ---

func writeFloat32(w io.Writer, v float32) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func readFloat32(r io.Reader) (float32, error) {
	var v float32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

func writeInt32(w io.Writer, v int32) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func readInt32(r io.Reader) (int32, error) {
	var v int32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

func writeString(w io.Writer, s string) error {
	if err := writeInt32(w, int32(len(s))); err != nil {
		return err
	}
	_, err := w.Write([]byte(s))
	return err
}

func readString(r io.Reader) (string, error) {
	length, err := readInt32(r)
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	return string(buf), err
}

// vectorsPath returns the path to the vectors data file.
func vectorsPath(storagePath string) string {
	return filepath.Join(storagePath, "vectors.dat")
}

// metadataPath returns the path to the metadata file.
func metadataPath(storagePath string) string {
	return filepath.Join(storagePath, "metadata.dat")
}

// indexPath returns the path to the index file.
func indexPath(storagePath string) string {
	return filepath.Join(storagePath, "index.dat")
}
