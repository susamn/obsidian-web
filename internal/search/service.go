package search

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/susamn/obsidian-web/internal/indexing"
)

// ServiceStatus represents the current state of the search service
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

// SearchService manages search operations with lifecycle and event handling
type SearchService struct {
	ctx    context.Context
	cancel context.CancelFunc

	// Configuration
	vaultID string

	// Index reference
	index   bleve.Index
	indexMu sync.RWMutex

	// Status
	status   ServiceStatus
	statusMu sync.RWMutex

	// Event handling
	updateChan chan indexing.IndexUpdateEvent
	stopChan   chan struct{}
	wg         sync.WaitGroup

	// Metrics
	searchCount    int64
	lastSearchTime time.Time
	indexRefreshes int64
	metricsmu      sync.RWMutex
}

// SearchMetrics holds search service metrics
type SearchMetrics struct {
	Status         ServiceStatus
	SearchCount    int64
	LastSearchTime time.Time
	IndexRefreshes int64
	HasIndex       bool
}

// NewSearchService creates a new search service
func NewSearchService(ctx context.Context, vaultID string, index bleve.Index) *SearchService {
	serviceCtx, cancel := context.WithCancel(ctx)

	return &SearchService{
		ctx:        serviceCtx,
		cancel:     cancel,
		vaultID:    vaultID,
		index:      index,
		status:     StatusInitializing,
		updateChan: make(chan indexing.IndexUpdateEvent, 10), // Buffer for index updates
		stopChan:   make(chan struct{}),
	}
}

// Start starts the search service
func (s *SearchService) Start() error {
	s.statusMu.Lock()
	if s.status != StatusInitializing {
		s.statusMu.Unlock()
		return nil // Already started
	}
	s.statusMu.Unlock()

	log.Printf("[%s] Starting search service...", s.vaultID)

	// Start event processor (always start it to receive index updates)
	s.wg.Add(1)
	go s.processIndexUpdates()

	// Check if we have an index
	s.indexMu.RLock()
	hasIndex := s.index != nil
	s.indexMu.RUnlock()

	if hasIndex {
		s.setStatus(StatusReady)
		log.Printf("[%s] Search service ready", s.vaultID)
	} else {
		// Not ready yet, waiting for index - stay in Initializing status
		// Will transition to Ready when we receive index update notification
		log.Printf("[%s] Search service waiting for index", s.vaultID)
	}

	return nil
}

// Stop stops the search service
func (s *SearchService) Stop() error {
	s.statusMu.Lock()
	if s.status == StatusStopped {
		s.statusMu.Unlock()
		return nil
	}
	s.statusMu.Unlock()

	log.Printf("[%s] Stopping search service...", s.vaultID)

	// Signal stop
	close(s.stopChan)

	// Cancel context
	s.cancel()

	// Wait for goroutines
	s.wg.Wait()

	s.setStatus(StatusStopped)
	log.Printf("[%s] Search service stopped", s.vaultID)

	return nil
}

// processIndexUpdates handles index update events
func (s *SearchService) processIndexUpdates() {
	defer s.wg.Done()

	log.Printf("[%s] Index update processor started", s.vaultID)

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("[%s] Index update processor stopped (context cancelled)", s.vaultID)
			return

		case <-s.stopChan:
			log.Printf("[%s] Index update processor stopped", s.vaultID)
			return

		case event, ok := <-s.updateChan:
			if !ok {
				log.Printf("[%s] Index update processor stopped (channel closed)", s.vaultID)
				return
			}

			// Process index update event
			s.processIndexUpdateEvent(event)
		}
	}
}

// processIndexUpdateEvent handles index update notifications
// For incremental updates: Just updates metrics, index is already updated in-place
// For rebuilds: Updates the index reference to point to the new index
func (s *SearchService) processIndexUpdateEvent(event indexing.IndexUpdateEvent) {
	// Update metrics
	s.metricsmu.Lock()
	s.indexRefreshes++
	refreshCount := s.indexRefreshes
	s.metricsmu.Unlock()

	// Handle based on event type
	if event.EventType == "rebuild" && event.NewIndex != nil {
		// Index was rebuilt - update our reference to the new index
		s.indexMu.Lock()
		s.index = event.NewIndex
		s.indexMu.Unlock()

		log.Printf("[%s] Index reference updated after rebuild (refresh #%d)", s.vaultID, refreshCount)

		// If we just got an index and were in error state, move to ready
		s.statusMu.Lock()
		if s.status == StatusError || s.status == StatusInitializing {
			s.status = StatusReady
			log.Printf("[%s] Search service now ready", s.vaultID)
		}
		s.statusMu.Unlock()
	} else {
		// Incremental update - index updated in-place, just log for metrics
		log.Printf("[%s] Index updated incrementally (refresh #%d)", s.vaultID, refreshCount)

		// If we have an index and were waiting, move to ready
		s.indexMu.RLock()
		hasIndex := s.index != nil
		s.indexMu.RUnlock()

		if hasIndex {
			s.statusMu.Lock()
			if s.status == StatusError || s.status == StatusInitializing {
				s.status = StatusReady
				log.Printf("[%s] Search service now ready", s.vaultID)
			}
			s.statusMu.Unlock()
		}
	}
}

// NotifyIndexUpdate sends an index update notification (non-blocking)
// This is called by IndexService when the index is updated.
// For incremental updates, event.NewIndex should be nil.
// For rebuilds, event.NewIndex should be the new index reference.
func (s *SearchService) NotifyIndexUpdate(event indexing.IndexUpdateEvent) {
	select {
	case s.updateChan <- event:
		// Queued successfully
	case <-s.ctx.Done():
		// Service stopping
	default:
		// Channel full, skip this update (non-blocking)
		log.Printf("[%s] ⚠️  Search update channel full, skipping refresh", s.vaultID)
	}
}

// GetStatus returns the current service status
func (s *SearchService) GetStatus() ServiceStatus {
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return s.status
}

// setStatus sets the service status
func (s *SearchService) setStatus(status ServiceStatus) {
	s.statusMu.Lock()
	s.status = status
	s.statusMu.Unlock()
}

// GetMetrics returns service metrics
func (s *SearchService) GetMetrics() SearchMetrics {
	s.statusMu.RLock()
	status := s.status
	s.statusMu.RUnlock()

	s.metricsmu.RLock()
	searchCount := s.searchCount
	lastSearchTime := s.lastSearchTime
	indexRefreshes := s.indexRefreshes
	s.metricsmu.RUnlock()

	s.indexMu.RLock()
	hasIndex := s.index != nil
	s.indexMu.RUnlock()

	return SearchMetrics{
		Status:         status,
		SearchCount:    searchCount,
		LastSearchTime: lastSearchTime,
		IndexRefreshes: indexRefreshes,
		HasIndex:       hasIndex,
	}
}

// getIndex returns the current index (thread-safe)
func (s *SearchService) getIndex() bleve.Index {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()
	return s.index
}

// recordSearch increments search count and updates timestamp
func (s *SearchService) recordSearch() {
	s.metricsmu.Lock()
	s.searchCount++
	s.lastSearchTime = time.Now()
	s.metricsmu.Unlock()
}

// Search Methods (all use getIndex() and recordSearch())

// SearchByText performs full-text search across all indexed content
func (s *SearchService) SearchByText(queryStr string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	q := bleve.NewMatchQuery(queryStr)
	search := bleve.NewSearchRequest(q)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByTag searches for documents with a specific tag
func (s *SearchService) SearchByTag(tag string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	q := bleve.NewMatchQuery(tag)
	q.SetField("tags")
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByMultipleTags searches for documents matching all specified tags (AND)
func (s *SearchService) SearchByMultipleTags(tags []string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	queries := make([]query.Query, len(tags))
	for i, tag := range tags {
		q := bleve.NewMatchQuery(tag)
		q.SetField("tags")
		queries[i] = q
	}

	q := bleve.NewConjunctionQuery(queries...)
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByWikilink searches for documents that contain a specific wikilink
func (s *SearchService) SearchByWikilink(wikilink string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	q := bleve.NewMatchQuery(wikilink)
	q.SetField("wikilinks")
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByBacklinks finds all documents that link to a specific note
func (s *SearchService) SearchByBacklinks(noteName string) (*bleve.SearchResult, error) {
	return s.SearchByWikilink(noteName)
}

// SearchByTagsOR searches for documents matching ANY of the specified tags (OR logic)
func (s *SearchService) SearchByTagsOR(tags []string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	queries := make([]query.Query, len(tags))
	for i, tag := range tags {
		q := bleve.NewMatchQuery(tag)
		q.SetField("tags")
		queries[i] = q
	}

	q := bleve.NewDisjunctionQuery(queries...)
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByMultipleWikilinks searches for documents containing all specified wikilinks (AND)
func (s *SearchService) SearchByMultipleWikilinks(wikilinks []string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	queries := make([]query.Query, len(wikilinks))
	for i, wikilink := range wikilinks {
		q := bleve.NewMatchQuery(wikilink)
		q.SetField("wikilinks")
		queries[i] = q
	}

	q := bleve.NewConjunctionQuery(queries...)
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByWikilinksOR searches for documents containing ANY of the specified wikilinks (OR)
func (s *SearchService) SearchByWikilinksOR(wikilinks []string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	queries := make([]query.Query, len(wikilinks))
	for i, wikilink := range wikilinks {
		q := bleve.NewMatchQuery(wikilink)
		q.SetField("wikilinks")
		queries[i] = q
	}

	q := bleve.NewDisjunctionQuery(queries...)
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByTitleOnly searches only in the title field
func (s *SearchService) SearchByTitleOnly(queryStr string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	q := bleve.NewMatchQuery(queryStr)
	q.SetField("title")
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// FuzzySearch performs fuzzy text search (allows typos/misspellings)
func (s *SearchService) FuzzySearch(queryStr string, fuzziness int) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	q := bleve.NewFuzzyQuery(queryStr)
	q.Fuzziness = fuzziness
	search := bleve.NewSearchRequest(q)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// PhraseSearch searches for an exact phrase
func (s *SearchService) PhraseSearch(phrase string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	q := bleve.NewMatchPhraseQuery(phrase)
	search := bleve.NewSearchRequest(q)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// PrefixSearch searches for terms starting with a prefix
func (s *SearchService) PrefixSearch(prefix string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	q := bleve.NewPrefixQuery(prefix)
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// AdvancedSearch performs combined text and tag search
func (s *SearchService) AdvancedSearch(text string, tags []string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	queries := []query.Query{}

	if text != "" {
		queries = append(queries, bleve.NewMatchQuery(text))
	}

	for _, tag := range tags {
		q := bleve.NewMatchQuery(tag)
		q.SetField("tags")
		queries = append(queries, q)
	}

	q := bleve.NewConjunctionQuery(queries...)
	search := bleve.NewSearchRequest(q)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchCombined performs a comprehensive search with text, tags, and wikilinks
func (s *SearchService) SearchCombined(text string, tags []string, wikilinks []string) (*bleve.SearchResult, error) {
	index := s.getIndex()
	if index == nil {
		return nil, fmt.Errorf("search service not ready: index not available")
	}

	s.recordSearch()

	queries := []query.Query{}

	if text != "" {
		queries = append(queries, bleve.NewMatchQuery(text))
	}

	for _, tag := range tags {
		q := bleve.NewMatchQuery(tag)
		q.SetField("tags")
		queries = append(queries, q)
	}

	for _, wikilink := range wikilinks {
		q := bleve.NewMatchQuery(wikilink)
		q.SetField("wikilinks")
		queries = append(queries, q)
	}

	if len(queries) == 0 {
		return &bleve.SearchResult{}, nil
	}

	q := bleve.NewConjunctionQuery(queries...)
	search := bleve.NewSearchRequest(q)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}
