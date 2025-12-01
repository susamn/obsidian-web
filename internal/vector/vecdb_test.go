package vector

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(DefaultConfig(128, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if db.Count() != 0 {
		t.Errorf("expected empty database, got %d vectors", db.Count())
	}
}

func TestOpenInvalidConfig(t *testing.T) {
	dir := t.TempDir()

	// Test zero dimension
	_, err := Open(Config{Dimension: 0, StoragePath: dir})
	if err == nil {
		t.Error("expected error for zero dimension")
	}

	// Test empty storage path
	_, err = Open(Config{Dimension: 128, StoragePath: ""})
	if err == nil {
		t.Error("expected error for empty storage path")
	}
}

func TestTrainAndQuery(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Train some vectors
	vectors := []struct {
		id     string
		vector []float32
		meta   map[string]string
	}{
		{"vec1", []float32{1, 0, 0, 0}, map[string]string{"type": "a"}},
		{"vec2", []float32{0, 1, 0, 0}, map[string]string{"type": "b"}},
		{"vec3", []float32{0, 0, 1, 0}, map[string]string{"type": "a"}},
		{"vec4", []float32{1, 1, 0, 0}, map[string]string{"type": "b"}},
	}

	for _, v := range vectors {
		if err := db.Train(v.id, v.vector, v.meta); err != nil {
			t.Fatalf("failed to train vector %s: %v", v.id, err)
		}
	}

	if db.Count() != 4 {
		t.Errorf("expected 4 vectors, got %d", db.Count())
	}

	// Query for similar vectors
	query := []float32{1, 0.1, 0, 0}
	results, err := db.Query(query, 2)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// First result should be vec1 (closest to query)
	if results[0].ID != "vec1" {
		t.Errorf("expected first result to be vec1, got %s", results[0].ID)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Add vectors
	db.Train("vec1", []float32{1, 0, 0, 0}, nil)
	db.Train("vec2", []float32{0, 1, 0, 0}, nil)

	if db.Count() != 2 {
		t.Errorf("expected 2 vectors, got %d", db.Count())
	}

	// Delete one
	if err := db.Delete("vec1"); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if db.Count() != 1 {
		t.Errorf("expected 1 vector after delete, got %d", db.Count())
	}

	if db.Has("vec1") {
		t.Error("vec1 should not exist after delete")
	}

	// Delete non-existent should error
	if err := db.Delete("vec1"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()

	// Create and populate database
	db, err := Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	db.Train("vec1", []float32{1, 0, 0, 0}, map[string]string{"name": "first"})
	db.Train("vec2", []float32{0, 1, 0, 0}, map[string]string{"name": "second"})
	db.Close()

	// Reopen database
	db, err = Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to reopen database: %v", err)
	}
	defer db.Close()

	if db.Count() != 2 {
		t.Errorf("expected 2 vectors after reopen, got %d", db.Count())
	}

	// Verify data
	v, err := db.Get("vec1")
	if err != nil {
		t.Fatalf("failed to get vec1: %v", err)
	}

	if v.Metadata["name"] != "first" {
		t.Errorf("expected metadata 'first', got '%s'", v.Metadata["name"])
	}

	// Query should still work
	results, err := db.Query([]float32{1, 0, 0, 0}, 1)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if results[0].ID != "vec1" {
		t.Errorf("expected vec1, got %s", results[0].ID)
	}
}

func TestUpdate(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Train initial vector
	db.Train("vec1", []float32{1, 0, 0, 0}, map[string]string{"version": "1"})

	// Update vector
	db.Train("vec1", []float32{0, 1, 0, 0}, map[string]string{"version": "2"})

	if db.Count() != 1 {
		t.Errorf("expected 1 vector after update, got %d", db.Count())
	}

	v, _ := db.Get("vec1")
	if v.Metadata["version"] != "2" {
		t.Errorf("expected version 2, got %s", v.Metadata["version"])
	}
}

func TestQueryWithFilter(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Train vectors with different types
	db.Train("vec1", []float32{1, 0, 0, 0}, map[string]string{"type": "a"})
	db.Train("vec2", []float32{1, 0.1, 0, 0}, map[string]string{"type": "b"})
	db.Train("vec3", []float32{1, 0.2, 0, 0}, map[string]string{"type": "a"})

	// Query with filter for type "a"
	results, err := db.QueryWithFilter([]float32{1, 0, 0, 0}, 2, func(meta map[string]string) bool {
		return meta["type"] == "a"
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Metadata["type"] != "a" {
			t.Errorf("expected type 'a', got '%s'", r.Metadata["type"])
		}
	}
}

func TestBatchOperations(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Batch train
	vectors := []*Vector{
		{ID: "vec1", Data: []float32{1, 0, 0, 0}, Metadata: map[string]string{"idx": "1"}},
		{ID: "vec2", Data: []float32{0, 1, 0, 0}, Metadata: map[string]string{"idx": "2"}},
		{ID: "vec3", Data: []float32{0, 0, 1, 0}, Metadata: map[string]string{"idx": "3"}},
	}

	if err := db.TrainBatch(vectors); err != nil {
		t.Fatalf("batch train failed: %v", err)
	}

	if db.Count() != 3 {
		t.Errorf("expected 3 vectors, got %d", db.Count())
	}

	// Batch delete
	if err := db.DeleteBatch([]string{"vec1", "vec3"}); err != nil {
		t.Fatalf("batch delete failed: %v", err)
	}

	if db.Count() != 1 {
		t.Errorf("expected 1 vector after delete, got %d", db.Count())
	}

	if !db.Has("vec2") {
		t.Error("vec2 should still exist")
	}
}

func TestCompact(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Add many vectors
	for i := 0; i < 100; i++ {
		v := []float32{float32(i), float32(i), float32(i), float32(i)}
		db.Train(string(rune('a'+i%26))+string(rune('0'+i/26)), v, nil)
	}

	// Delete half
	for i := 0; i < 50; i++ {
		db.Delete(string(rune('a'+i%26)) + string(rune('0'+i/26)))
	}

	// Get file size before compact
	vectorsPath := filepath.Join(dir, vectorsFileName)
	infoBefore, _ := os.Stat(vectorsPath)
	sizeBefore := infoBefore.Size()

	// Compact
	if err := db.Compact(); err != nil {
		t.Fatalf("compact failed: %v", err)
	}

	// File should be smaller
	infoAfter, _ := os.Stat(vectorsPath)
	sizeAfter := infoAfter.Size()

	if sizeAfter >= sizeBefore {
		t.Errorf("expected smaller file after compact: before=%d, after=%d", sizeBefore, sizeAfter)
	}

	// Data should still be accessible
	if db.Count() != 50 {
		t.Errorf("expected 50 vectors after compact, got %d", db.Count())
	}
}

func TestDistanceFunctions(t *testing.T) {
	a := []float32{1, 0, 0, 0}
	b := []float32{0, 1, 0, 0}
	c := []float32{1, 0, 0, 0}

	// Cosine distance
	cosAB := CosineDistance(a, b)
	cosAC := CosineDistance(a, c)
	if cosAB <= cosAC {
		t.Errorf("cosine: expected a-b > a-c, got %f <= %f", cosAB, cosAC)
	}
	if cosAC != 0 {
		t.Errorf("cosine: expected a-c = 0, got %f", cosAC)
	}

	// Euclidean distance
	eucAB := EuclideanDistance(a, b)
	eucAC := EuclideanDistance(a, c)
	expectedEuc := float32(math.Sqrt(2))
	if math.Abs(float64(eucAB-expectedEuc)) > 0.001 {
		t.Errorf("euclidean: expected %f, got %f", expectedEuc, eucAB)
	}
	if eucAC != 0 {
		t.Errorf("euclidean: expected a-c = 0, got %f", eucAC)
	}

	// Manhattan distance
	manAB := ManhattanDistance(a, b)
	if manAB != 2 {
		t.Errorf("manhattan: expected 2, got %f", manAB)
	}
}

func TestDimensionMismatch(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Wrong dimension should fail
	err = db.Train("vec1", []float32{1, 0, 0}, nil)
	if err == nil {
		t.Error("expected dimension mismatch error")
	}

	// Add correct vector first
	db.Train("vec1", []float32{1, 0, 0, 0}, nil)

	// Query with wrong dimension
	_, err = db.Query([]float32{1, 0, 0}, 1)
	if err == nil {
		t.Error("expected dimension mismatch error on query")
	}
}

func TestClosedDatabase(t *testing.T) {
	dir := t.TempDir()

	db, err := Open(DefaultConfig(4, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	db.Close()

	// All operations should fail
	err = db.Train("vec1", []float32{1, 0, 0, 0}, nil)
	if err != ErrDatabaseClosed {
		t.Errorf("expected ErrDatabaseClosed, got %v", err)
	}

	_, err = db.Query([]float32{1, 0, 0, 0}, 1)
	if err != ErrDatabaseClosed {
		t.Errorf("expected ErrDatabaseClosed on query, got %v", err)
	}

	err = db.Delete("vec1")
	if err != ErrDatabaseClosed {
		t.Errorf("expected ErrDatabaseClosed on delete, got %v", err)
	}
}

func TestLargeScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large scale test in short mode")
	}

	dir := t.TempDir()
	dim := 128

	db, err := Open(DefaultConfig(dim, dir))
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Insert 1000 vectors
	n := 1000
	for i := 0; i < n; i++ {
		v := make([]float32, dim)
		for j := range v {
			v[j] = rand.Float32()
		}
		if err := db.Train(string(rune(i)), v, nil); err != nil {
			t.Fatalf("failed to train vector %d: %v", i, err)
		}
	}

	if db.Count() != n {
		t.Errorf("expected %d vectors, got %d", n, db.Count())
	}

	// Query
	query := make([]float32, dim)
	for i := range query {
		query[i] = rand.Float32()
	}

	results, err := db.Query(query, 10)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}

	// Verify results are sorted by distance
	for i := 1; i < len(results); i++ {
		if results[i].Distance < results[i-1].Distance {
			t.Errorf("results not sorted: %f < %f", results[i].Distance, results[i-1].Distance)
		}
	}
}

func BenchmarkTrain(b *testing.B) {
	dir := b.TempDir()
	dim := 128

	db, err := Open(DefaultConfig(dim, dir))
	if err != nil {
		b.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	v := make([]float32, dim)
	for i := range v {
		v[i] = rand.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Train(string(rune(i)), v, nil)
	}
}

func BenchmarkQuery(b *testing.B) {
	dir := b.TempDir()
	dim := 128

	db, err := Open(DefaultConfig(dim, dir))
	if err != nil {
		b.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Insert vectors
	for i := 0; i < 10000; i++ {
		v := make([]float32, dim)
		for j := range v {
			v[j] = rand.Float32()
		}
		db.Train(string(rune(i)), v, nil)
	}

	query := make([]float32, dim)
	for i := range query {
		query[i] = rand.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Query(query, 10)
	}
}
