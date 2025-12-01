package vector

import (
	"fmt"
	"log"
	"math/rand"
)

func main() {
	// ============================================================
	// Example: Embeddable Vector Database for Go Applications
	// ============================================================

	// 1. Open/Create Database
	// --------------------------------------------------------
	// Configure with dimension size and storage path
	config := DefaultConfig(128, "./my_vectors")

	// You can customize the config:
	// config.Distance = EuclideanDistance  // Default is CosineDistance
	// config.M = 32                               // HNSW max connections (default: 16)
	// config.EfConstruction = 400                 // Construction quality (default: 200)
	// config.EfSearch = 100                       // Search quality (default: 50)

	db, err := Open(config)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	fmt.Println("✓ Database opened successfully")

	// ============================================================
	// 2. TRAIN API - Adding/Updating Vectors
	// ============================================================

	// Train a single vector with metadata
	vector1 := generateVector(128)
	metadata1 := map[string]string{
		"title":    "Introduction to Machine Learning",
		"category": "tech",
		"author":   "Jane Doe",
	}

	err = db.Train("doc-001", vector1, metadata1)
	if err != nil {
		log.Fatalf("Failed to train: %v", err)
	}
	fmt.Println("✓ Trained single vector")

	// Train without metadata
	vector2 := generateVector(128)
	err = db.Train("doc-002", vector2, nil)
	if err != nil {
		log.Fatalf("Failed to train: %v", err)
	}

	// Batch training for efficiency
	batchVectors := []*Vector{
		{
			ID:   "doc-003",
			Data: generateVector(128),
			Metadata: map[string]string{
				"title":    "Deep Learning Fundamentals",
				"category": "tech",
			},
		},
		{
			ID:   "doc-004",
			Data: generateVector(128),
			Metadata: map[string]string{
				"title":    "Natural Language Processing",
				"category": "tech",
			},
		},
		{
			ID:   "doc-005",
			Data: generateVector(128),
			Metadata: map[string]string{
				"title":    "Cooking Italian Food",
				"category": "lifestyle",
			},
		},
	}

	err = db.TrainBatch(batchVectors)
	if err != nil {
		log.Fatalf("Batch train failed: %v", err)
	}
	fmt.Printf("✓ Batch trained %d vectors\n", len(batchVectors))

	// Update existing vector (same ID, new data)
	updatedVector := generateVector(128)
	updatedMeta := map[string]string{
		"title":    "Introduction to Machine Learning (Updated)",
		"category": "tech",
		"author":   "Jane Doe",
		"version":  "2.0",
	}
	err = db.Train("doc-001", updatedVector, updatedMeta)
	if err != nil {
		log.Fatalf("Failed to update: %v", err)
	}
	fmt.Println("✓ Updated existing vector")

	fmt.Printf("Total vectors in database: %d\n\n", db.Count())

	// ============================================================
	// 3. QUERY API - Searching for Similar Vectors
	// ============================================================

	// Basic query - find k nearest neighbors
	queryVector := generateVector(128)
	results, err := db.Query(queryVector, 3)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Println("Query Results (top 3):")
	for i, r := range results {
		fmt.Printf("  %d. ID: %s, Distance: %.4f, Title: %s\n",
			i+1, r.ID, r.Distance, r.Metadata["title"])
	}
	fmt.Println()

	// Query with metadata filter
	techResults, err := db.QueryWithFilter(queryVector, 10, func(meta map[string]string) bool {
		return meta["category"] == "tech"
	})
	if err != nil {
		log.Fatalf("Filtered query failed: %v", err)
	}

	fmt.Println("Filtered Query Results (category='tech'):")
	for i, r := range techResults {
		fmt.Printf("  %d. ID: %s, Distance: %.4f, Category: %s\n",
			i+1, r.ID, r.Distance, r.Metadata["category"])
	}
	fmt.Println()

	// Get a specific vector by ID
	vec, err := db.Get("doc-001")
	if err != nil {
		log.Printf("Vector not found: %v", err)
	} else {
		fmt.Printf("Got vector doc-001: dimension=%d, title=%s\n",
			len(vec.Data), vec.Metadata["title"])
	}

	// Check if vector exists
	if db.Has("doc-001") {
		fmt.Println("✓ doc-001 exists")
	}
	fmt.Println()

	// ============================================================
	// 4. DELETE API - Removing Vectors
	// ============================================================

	// Delete single vector
	err = db.Delete("doc-002")
	if err != nil {
		log.Fatalf("Delete failed: %v", err)
	}
	fmt.Println("✓ Deleted doc-002")

	// Batch delete
	err = db.DeleteBatch([]string{"doc-003", "doc-004"})
	if err != nil {
		log.Fatalf("Batch delete failed: %v", err)
	}
	fmt.Println("✓ Batch deleted doc-003 and doc-004")

	fmt.Printf("Vectors remaining: %d\n\n", db.Count())

	// ============================================================
	// 5. MAINTENANCE - Compaction
	// ============================================================

	// Compact reclaims disk space from deleted vectors
	err = db.Compact()
	if err != nil {
		log.Fatalf("Compact failed: %v", err)
	}
	fmt.Println("✓ Database compacted")

	// ============================================================
	// 6. DIFFERENT DISTANCE FUNCTIONS
	// ============================================================

	fmt.Println("\n--- Distance Function Examples ---")

	a := []float32{1, 0, 0, 0}
	b := []float32{0.5, 0.5, 0, 0}

	fmt.Printf("Vector A: %v\n", a)
	fmt.Printf("Vector B: %v\n", b)
	fmt.Printf("Cosine Distance:     %.4f\n", CosineDistance(a, b))
	fmt.Printf("Euclidean Distance:  %.4f\n", EuclideanDistance(a, b))
	fmt.Printf("Manhattan Distance:  %.4f\n", ManhattanDistance(a, b))
	fmt.Printf("Dot Product Distance: %.4f\n", DotProductDistance(a, b))

	fmt.Println("\n✓ Example completed successfully!")
}

// generateVector creates a random normalized vector
func generateVector(dim int) []float32 {
	v := make([]float32, dim)
	var norm float32
	for i := range v {
		v[i] = rand.Float32()*2 - 1 // Range [-1, 1]
		norm += v[i] * v[i]
	}
	// Normalize
	norm = float32(1.0 / float64(norm))
	for i := range v {
		v[i] *= norm
	}
	return v
}
