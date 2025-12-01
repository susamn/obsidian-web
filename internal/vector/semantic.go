package vector

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrNoEmbedder   = errors.New("no embedder configured")
	ErrEmptyContent = errors.New("empty content")
	ErrAPIError     = errors.New("embedding API error")
	ErrNoResults    = errors.New("no results found")
)

// Embedder is the interface for text embedding providers.
type Embedder interface {
	// Embed converts text into a vector embedding.
	Embed(ctx context.Context, text string) ([]float32, error)
	// EmbedBatch converts multiple texts into embeddings.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	// Dimension returns the embedding dimension.
	Dimension() int
}

// Config holds configuration for the semantic search engine.
type Config struct {
	// StoragePath is where the vector database is stored.
	StoragePath string
	// Embedder is the embedding provider to use.
	Embedder Embedder
	// ChunkSize is the max characters per chunk (default: 1000).
	ChunkSize int
	// ChunkOverlap is the overlap between chunks (default: 200).
	ChunkOverlap int
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig(storagePath string, embedder Embedder) Config {
	return Config{
		StoragePath:  storagePath,
		Embedder:     embedder,
		ChunkSize:    1000,
		ChunkOverlap: 200,
	}
}

// Document represents a source document.
type Document struct {
	ID       string            // Unique identifier
	Content  string            // Full text content
	Metadata map[string]string // Custom metadata (title, source, etc.)
	FilePath string            // Original file path (if from file)
}

// SearchResult represents a search result.
type SearchResult struct {
	DocumentID string            // Source document ID
	ChunkID    string            // Specific chunk ID
	Content    string            // The matching text chunk
	Score      float32           // Similarity score (lower is better for distance)
	Metadata   map[string]string // Document metadata
}

// Engine is the main semantic search engine.
type Engine struct {
	mu       sync.RWMutex
	config   Config
	db       *DB
	embedder Embedder
	// Track document -> chunks mapping
	docChunks map[string][]string
}

// New creates a new semantic search engine.
func New(config Config) (*Engine, error) {
	if config.Embedder == nil {
		return nil, ErrNoEmbedder
	}
	if config.ChunkSize <= 0 {
		config.ChunkSize = 1000
	}
	if config.ChunkOverlap <= 0 {
		config.ChunkOverlap = 200
	}
	if config.ChunkOverlap >= config.ChunkSize {
		config.ChunkOverlap = config.ChunkSize / 5
	}

	// Open vector database
	dbConfig := DefaultConfig(config.Embedder.Dimension(), config.StoragePath)
	db, err := Open(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open vector database: %w", err)
	}

	engine := &Engine{
		config:    config,
		db:        db,
		embedder:  config.Embedder,
		docChunks: make(map[string][]string),
	}

	// Load existing document-chunk mappings
	if err := engine.loadDocChunks(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load document mappings: %w", err)
	}

	return engine, nil
}

// loadDocChunks loads the document-chunk mapping from disk.
func (e *Engine) loadDocChunks() error {
	path := filepath.Join(e.config.StoragePath, "doc_chunks.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &e.docChunks)
}

// saveDocChunks persists the document-chunk mapping.
func (e *Engine) saveDocChunks() error {
	path := filepath.Join(e.config.StoragePath, "doc_chunks.json")
	data, err := json.Marshal(e.docChunks)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Train adds a document to the search index.
// The document is chunked, embedded, and stored.
func (e *Engine) Train(ctx context.Context, doc *Document) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if doc.Content == "" {
		return ErrEmptyContent
	}

	// Remove existing chunks for this document
	if existingChunks, ok := e.docChunks[doc.ID]; ok {
		e.db.DeleteBatch(existingChunks)
	}

	// Chunk the document
	chunks := e.chunkText(doc.Content)
	if len(chunks) == 0 {
		return ErrEmptyContent
	}

	// Embed all chunks
	embeddings, err := e.embedder.EmbedBatch(ctx, chunks)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}

	// Store chunks
	vectors := make([]*Vector, len(chunks))
	chunkIDs := make([]string, len(chunks))

	for i, chunk := range chunks {
		chunkID := fmt.Sprintf("%s_chunk_%d", doc.ID, i)
		chunkIDs[i] = chunkID

		meta := make(map[string]string)
		for k, v := range doc.Metadata {
			meta[k] = v
		}
		meta["_doc_id"] = doc.ID
		meta["_chunk_idx"] = fmt.Sprintf("%d", i)
		meta["_chunk_count"] = fmt.Sprintf("%d", len(chunks))
		meta["_content"] = chunk
		if doc.FilePath != "" {
			meta["_file_path"] = doc.FilePath
		}

		vectors[i] = &Vector{
			ID:       chunkID,
			Data:     embeddings[i],
			Metadata: meta,
		}
	}

	if err := e.db.TrainBatch(vectors); err != nil {
		return fmt.Errorf("failed to store vectors: %w", err)
	}

	// Update document mapping
	e.docChunks[doc.ID] = chunkIDs
	if err := e.saveDocChunks(); err != nil {
		return fmt.Errorf("failed to save mappings: %w", err)
	}

	return nil
}

// TrainFile reads and trains on a text or markdown file.
func (e *Engine) TrainFile(ctx context.Context, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Generate document ID from file path
	hash := sha256.Sum256([]byte(filePath))
	docID := hex.EncodeToString(hash[:8])

	doc := &Document{
		ID:       docID,
		Content:  string(content),
		FilePath: filePath,
		Metadata: map[string]string{
			"filename": filepath.Base(filePath),
			"ext":      filepath.Ext(filePath),
		},
	}

	return e.Train(ctx, doc)
}

// TrainDirectory trains on all text/markdown files in a directory.
func (e *Engine) TrainDirectory(ctx context.Context, dirPath string, extensions []string) (int, error) {
	if len(extensions) == 0 {
		extensions = []string{".txt", ".md", ".markdown", ".text"}
	}

	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[strings.ToLower(ext)] = true
	}

	var count int
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !extMap[ext] {
			return nil
		}

		if err := e.TrainFile(ctx, path); err != nil {
			return fmt.Errorf("failed to train %s: %w", path, err)
		}
		count++
		return nil
	})

	return count, err
}

// Search performs a semantic search with a natural language query.
func (e *Engine) Search(ctx context.Context, query string, k int) ([]SearchResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if query == "" {
		return nil, ErrEmptyContent
	}
	if k <= 0 {
		k = 10
	}

	// Embed the query
	embedding, err := e.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query embedding failed: %w", err)
	}

	// Search
	results, err := e.db.Query(embedding, k)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	// Convert to SearchResults
	searchResults := make([]SearchResult, len(results))
	for i, r := range results {
		searchResults[i] = SearchResult{
			DocumentID: r.Metadata["_doc_id"],
			ChunkID:    r.ID,
			Content:    r.Metadata["_content"],
			Score:      r.Distance,
			Metadata:   r.Metadata,
		}
	}

	return searchResults, nil
}

// Ask is a convenience method that searches and returns a formatted answer.
// It returns the most relevant chunks that might answer the question.
func (e *Engine) Ask(ctx context.Context, question string) (string, []SearchResult, error) {
	results, err := e.Search(ctx, question, 5)
	if err != nil {
		return "", nil, err
	}

	// Build context from results
	var sb strings.Builder
	sb.WriteString("Based on my trained data, here are the most relevant sections:\n\n")

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("--- Result %d (Score: %.4f) ---\n", i+1, r.Score))
		if filename, ok := r.Metadata["filename"]; ok {
			sb.WriteString(fmt.Sprintf("Source: %s\n", filename))
		}
		sb.WriteString(r.Content)
		sb.WriteString("\n\n")
	}

	return sb.String(), results, nil
}

// HasContent checks if there's any content related to a topic.
func (e *Engine) HasContent(ctx context.Context, topic string) (bool, []SearchResult, error) {
	results, err := e.Search(ctx, topic, 3)
	if errors.Is(err, ErrNoResults) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	// Consider it a match if score is below threshold
	// For cosine distance, lower is better; 0.5 is a reasonable threshold
	const threshold = 0.5
	hasRelevant := false
	var relevant []SearchResult

	for _, r := range results {
		if r.Score < threshold {
			hasRelevant = true
			relevant = append(relevant, r)
		}
	}

	return hasRelevant, relevant, nil
}

// Delete removes a document and all its chunks.
func (e *Engine) Delete(docID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	chunks, ok := e.docChunks[docID]
	if !ok {
		return ErrNotFound
	}

	if err := e.db.DeleteBatch(chunks); err != nil {
		return err
	}

	delete(e.docChunks, docID)
	return e.saveDocChunks()
}

// Stats returns statistics about the index.
func (e *Engine) Stats() map[string]int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]int{
		"documents": len(e.docChunks),
		"chunks":    e.db.Count(),
	}
}

// Close closes the search engine.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.db.Close()
}

// chunkText splits text into overlapping chunks.
func (e *Engine) chunkText(text string) []string {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return nil
	}

	// If text is small enough, return as single chunk
	if len(text) <= e.config.ChunkSize {
		return []string{text}
	}

	var chunks []string
	runes := []rune(text)
	start := 0

	for start < len(runes) {
		end := start + e.config.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}

		// Try to break at a sentence or paragraph boundary
		if end < len(runes) {
			// Look for paragraph break
			for i := end; i > start+e.config.ChunkSize/2; i-- {
				if runes[i] == '\n' && i+1 < len(runes) && runes[i+1] == '\n' {
					end = i
					break
				}
			}
			// Look for sentence break
			if end == start+e.config.ChunkSize {
				for i := end; i > start+e.config.ChunkSize/2; i-- {
					if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
						end = i + 1
						break
					}
				}
			}
		}

		chunk := strings.TrimSpace(string(runes[start:end]))
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}

		// Move start with overlap
		start = end - e.config.ChunkOverlap
		if start < 0 {
			start = 0
		}
		if start >= end {
			start = end
		}
	}

	return chunks
}

// ============================================================
// Embedding Providers
// ============================================================

// OpenAIEmbedder uses OpenAI's embedding API.
type OpenAIEmbedder struct {
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
}

// NewOpenAIEmbedder creates an OpenAI embedder.
// model: "text-embedding-3-small" (1536 dim) or "text-embedding-3-large" (3072 dim)
func NewOpenAIEmbedder(apiKey string, model string) *OpenAIEmbedder {
	dim := 1536
	if model == "text-embedding-3-large" {
		dim = 3072
	} else if model == "" {
		model = "text-embedding-3-small"
	}

	return &OpenAIEmbedder{
		apiKey:    apiKey,
		model:     model,
		dimension: dim,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := map[string]interface{}{
		"input": texts,
		"model": e.model,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: %s", ErrAPIError, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
}

// OllamaEmbedder uses local Ollama for embeddings.
type OllamaEmbedder struct {
	baseURL    string
	model      string
	dimension  int
	httpClient *http.Client
}

// NewOllamaEmbedder creates an Ollama embedder.
// Common models: "nomic-embed-text" (768 dim), "mxbai-embed-large" (1024 dim)
func NewOllamaEmbedder(baseURL, model string, dimension int) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaEmbedder{
		baseURL:   baseURL,
		model:     model,
		dimension: dimension,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (e *OllamaEmbedder) Dimension() int {
	return e.dimension
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":  e.model,
		"prompt": text,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: %s", ErrAPIError, string(body))
	}

	var result struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	embedding := make([]float32, len(result.Embedding))
	for i, v := range result.Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// SimpleHashEmbedder is a basic embedder for testing (not for production!).
// It creates deterministic embeddings using hashing - useful for testing only.
type SimpleHashEmbedder struct {
	dimension int
}

func NewSimpleHashEmbedder(dimension int) *SimpleHashEmbedder {
	return &SimpleHashEmbedder{dimension: dimension}
}

func (e *SimpleHashEmbedder) Dimension() int {
	return e.dimension
}

func (e *SimpleHashEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Tokenize by words
	words := strings.Fields(strings.ToLower(text))
	embedding := make([]float32, e.dimension)

	for _, word := range words {
		hash := sha256.Sum256([]byte(word))
		for i := 0; i < e.dimension && i < 32; i++ {
			embedding[i%e.dimension] += float32(hash[i]) / 255.0
		}
	}

	// Normalize
	var norm float32
	for _, v := range embedding {
		norm += v * v
	}
	if norm > 0 {
		norm = float32(1.0 / float64(norm))
		for i := range embedding {
			embedding[i] *= norm
		}
	}

	return embedding, nil
}

func (e *SimpleHashEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// VoyageAIEmbedder uses Voyage AI's embedding API.
type VoyageAIEmbedder struct {
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
}

// NewVoyageAIEmbedder creates a Voyage AI embedder.
// Models: "voyage-3" (1024 dim), "voyage-3-lite" (512 dim), "voyage-code-3" (1024 dim)
func NewVoyageAIEmbedder(apiKey string, model string) *VoyageAIEmbedder {
	dim := 1024
	if model == "voyage-3-lite" {
		dim = 512
	} else if model == "" {
		model = "voyage-3"
	}

	return &VoyageAIEmbedder{
		apiKey:    apiKey,
		model:     model,
		dimension: dim,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *VoyageAIEmbedder) Dimension() int {
	return e.dimension
}

func (e *VoyageAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

func (e *VoyageAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := map[string]interface{}{
		"input": texts,
		"model": e.model,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.voyageai.com/v1/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: %s", ErrAPIError, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	embeddings := make([][]float32, len(texts))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, nil
}

// Helper to read file with progress
func ReadFileWithProgress(path string, progress func(int64, int64)) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	reader := bufio.NewReader(f)
	var read int64

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		buf.WriteString(line)
		read += int64(len(line))

		if progress != nil {
			progress(read, info.Size())
		}

		if err == io.EOF {
			break
		}
	}

	return buf.String(), nil
}
