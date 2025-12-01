# VecDB - Embeddable Vector Database for Go

A high-performance, embeddable vector database for Go applications with disk persistence and efficient similarity search using the HNSW (Hierarchical Navigable Small World) algorithm.

## Features

- **Embeddable**: Use directly in your Go application with no external dependencies
- **Disk Persistence**: Data survives application restarts
- **HNSW Index**: Fast approximate nearest neighbor search
- **Multiple Distance Functions**: Cosine, Euclidean, Manhattan, Dot Product
- **Metadata Support**: Store and filter by custom metadata
- **Thread-Safe**: Safe for concurrent access
- **Batch Operations**: Efficient bulk inserts and deletes
- **Compaction**: Reclaim disk space from deleted vectors

## Installation

```bash
go get github.com/yourusername/vecdb
```

## Quick Start

```go
package main

import (
    "log"
    "github.com/yourusername/vecdb"
)

func main() {
    // Open database with 128-dimensional vectors
    db, err := vecdb.Open(vecdb.DefaultConfig(128, "./vectors"))
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Train (add) a vector
    err = db.Train("doc1", []float32{0.1, 0.2, ...}, map[string]string{
        "title": "My Document",
    })

    // Query for similar vectors
    results, err := db.Query(queryVector, 10) // top 10 results

    // Delete a vector
    err = db.Delete("doc1")
}
```

## API Reference

### Opening a Database

```go
// With default configuration
config := vecdb.DefaultConfig(dimension int, storagePath string)
db, err := vecdb.Open(config)

// With custom configuration
config := vecdb.Config{
    Dimension:      128,                    // Vector dimension (required)
    StoragePath:    "./vectors",            // Storage directory (required)
    Distance:       vecdb.CosineDistance,   // Distance function
    M:              16,                     // HNSW max connections per layer
    EfConstruction: 200,                    // Construction quality parameter
    EfSearch:       50,                     // Search quality parameter
}
db, err := vecdb.Open(config)
```

### Training (Adding/Updating) Vectors

```go
// Add a single vector with metadata
err := db.Train(id string, vector []float32, metadata map[string]string)

// Add a single vector without metadata
err := db.Train("vec1", vector, nil)

// Update existing vector (same ID overwrites)
err := db.Train("vec1", newVector, newMetadata)

// Batch add vectors
vectors := []*vecdb.Vector{
    {ID: "v1", Data: vec1, Metadata: meta1},
    {ID: "v2", Data: vec2, Metadata: meta2},
}
err := db.TrainBatch(vectors)
```

### Querying Vectors

```go
// Basic k-NN search
results, err := db.Query(queryVector []float32, k int)

// Returns []SearchResult:
// type SearchResult struct {
//     ID       string
//     Distance float32
//     Metadata map[string]string
// }

// Query with metadata filter
results, err := db.QueryWithFilter(queryVector, k, func(meta map[string]string) bool {
    return meta["category"] == "tech"
})

// Get specific vector by ID
vector, err := db.Get("vec1")

// Check if vector exists
exists := db.Has("vec1")

// Get vector count
count := db.Count()
```

### Deleting Vectors

```go
// Delete single vector
err := db.Delete("vec1")

// Batch delete
err := db.DeleteBatch([]string{"vec1", "vec2", "vec3"})
```

### Maintenance

```go
// Compact database (reclaim space from deleted vectors)
err := db.Compact()

// Close database
err := db.Close()
```

## Distance Functions

The library includes four distance functions:

| Function | Description | Best For |
|----------|-------------|----------|
| `CosineDistance` | 1 - cosine similarity | Text embeddings, normalized vectors |
| `EuclideanDistance` | L2 distance | General purpose |
| `ManhattanDistance` | L1 distance | High-dimensional sparse data |
| `DotProductDistance` | Negative dot product | Maximum inner product search |

```go
// Use different distance function
config := vecdb.DefaultConfig(128, "./vectors")
config.Distance = vecdb.EuclideanDistance
db, _ := vecdb.Open(config)
```

## HNSW Parameters

The HNSW algorithm has several tuning parameters:

| Parameter | Default | Description |
|-----------|---------|-------------|
| `M` | 16 | Max connections per node per layer. Higher = better recall, more memory |
| `EfConstruction` | 200 | Size of dynamic candidate list during indexing. Higher = slower build, better index |
| `EfSearch` | 50 | Size of dynamic candidate list during search. Higher = slower search, better recall |

```go
config := vecdb.DefaultConfig(128, "./vectors")
config.M = 32              // More connections
config.EfConstruction = 400 // Higher quality index
config.EfSearch = 100       // Better search recall
```

## Persistence

Data is automatically persisted to disk. The storage directory contains:

```
./vectors/
├── vectors.bin    # Raw vector data
├── metadata.gob   # Vector metadata
└── index.gob      # Offset index
```

After deleting many vectors, call `Compact()` to reclaim disk space.

## Thread Safety

All operations are thread-safe. The database uses read-write locks:
- Reads (`Query`, `Get`, `Has`, `Count`) can execute concurrently
- Writes (`Train`, `Delete`, `Compact`) are exclusive

## Error Handling

```go
var (
    vecdb.ErrNotFound          // Vector not found
    vecdb.ErrDimensionMismatch // Vector dimension doesn't match DB
    vecdb.ErrEmptyVector       // Empty vector provided
    vecdb.ErrDatabaseClosed    // Operation on closed database
    vecdb.ErrInvalidK          // Invalid k parameter
)
```

## Example: Building a Semantic Search Engine

```go
package main

import (
    "log"
    "github.com/yourusername/vecdb"
)

func main() {
    // Create database for 384-dim embeddings (e.g., sentence-transformers)
    db, _ := vecdb.Open(vecdb.DefaultConfig(384, "./search_index"))
    defer db.Close()

    // Index documents (vectors would come from an embedding model)
    documents := []struct {
        id      string
        content string
        vector  []float32
    }{
        {"doc1", "Machine learning basics", getEmbedding("Machine learning basics")},
        {"doc2", "Deep neural networks", getEmbedding("Deep neural networks")},
        {"doc3", "Cooking recipes", getEmbedding("Cooking recipes")},
    }

    for _, doc := range documents {
        db.Train(doc.id, doc.vector, map[string]string{
            "content": doc.content,
        })
    }

    // Search
    query := "AI and neural networks"
    queryVec := getEmbedding(query)
    
    results, _ := db.Query(queryVec, 5)
    
    for _, r := range results {
        log.Printf("%.4f - %s", r.Distance, r.Metadata["content"])
    }
}

func getEmbedding(text string) []float32 {
    // Use your preferred embedding model here
    // e.g., OpenAI, sentence-transformers, etc.
    return make([]float32, 384)
}
```

## Benchmarks

Approximate performance on a typical machine:

| Operation | Vectors | Time |
|-----------|---------|------|
| Train | 1 | ~100μs |
| Train | 10,000 | ~1s |
| Query (k=10) | 10,000 | ~1ms |
| Query (k=10) | 100,000 | ~5ms |

## License

MIT License
