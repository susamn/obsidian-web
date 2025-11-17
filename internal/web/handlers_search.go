package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/blevesearch/bleve/v2"
)

// SearchRequest represents a search request
type SearchRequest struct {
	Query     string   `json:"query"`
	Type      string   `json:"type,omitempty"`       // text, tag, wikilink, fuzzy, phrase, prefix
	Tags      []string `json:"tags,omitempty"`       // for tag search
	Wikilinks []string `json:"wikilinks,omitempty"`  // for wikilink search
	Limit     int      `json:"limit,omitempty"`      // max results
	TitleOnly bool     `json:"title_only,omitempty"` // search title only
}

// SearchResult represents a single search result
type SearchResult struct {
	ID        string                 `json:"id"`
	Score     float64                `json:"score"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Fragments map[string][]string    `json:"fragments,omitempty"`
}

// SearchResponse represents search results
type SearchResponse struct {
	Total   uint64         `json:"total"`
	Results []SearchResult `json:"results"`
	Took    string         `json:"took"`
}

// handleSearch godoc
// @Summary Search a vault
// @Description Search for notes in a vault
// @Tags search
// @Accept json
// @Produce json
// @Param vault path string true "Vault ID"
// @Param query body SearchRequest true "Search query"
// @Success 200 {object} SearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 405 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /api/v1/search/{vault} [post]
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract vault ID from path
	vaultID := s.extractVaultID(r.URL.Path, "/api/v1/search/")
	if vaultID == "" {
		writeError(w, http.StatusBadRequest, "Vault ID required")
		return
	}

	// Get vault
	v, ok := s.getVault(vaultID)
	if !ok {
		writeError(w, http.StatusNotFound, "Vault not found")
		return
	}

	// Check vault is active
	if !v.IsActive() {
		writeError(w, http.StatusServiceUnavailable, "Vault not active")
		return
	}

	// Parse search request
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Validate request
	if req.Query == "" && len(req.Tags) == 0 && len(req.Wikilinks) == 0 {
		writeError(w, http.StatusBadRequest, "Query, tags, or wikilinks required")
		return
	}

	// Set default limit
	if req.Limit == 0 {
		req.Limit = 50
	}

	// Get search service
	searchSvc := v.GetSearchService()
	if searchSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "Search service not available")
		return
	}

	// Execute search
	results, err := s.executeSearch(searchSvc, &req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Search failed: %v", err))
		return
	}

	// Return results
	writeSuccess(w, results)
}

// searchService interface for methods we need
type searchService interface {
	SearchByText(query string) (*bleve.SearchResult, error)
	SearchByTag(tag string) (*bleve.SearchResult, error)
	SearchByMultipleTags(tags []string) (*bleve.SearchResult, error)
	SearchByWikilink(wikilink string) (*bleve.SearchResult, error)
	SearchByMultipleWikilinks(wikilinks []string) (*bleve.SearchResult, error)
	SearchByTitleOnly(query string) (*bleve.SearchResult, error)
	FuzzySearch(query string, fuzziness int) (*bleve.SearchResult, error)
	PhraseSearch(phrase string) (*bleve.SearchResult, error)
	PrefixSearch(prefix string) (*bleve.SearchResult, error)
}

// executeSearch performs the actual search based on request type
func (s *Server) executeSearch(searchSvc searchService, req *SearchRequest) (*SearchResponse, error) {
	var result *bleve.SearchResult
	var err error

	// Execute search based on type
	switch req.Type {
	case "tag":
		if len(req.Tags) == 1 {
			result, err = searchSvc.SearchByTag(req.Tags[0])
		} else if len(req.Tags) > 1 {
			result, err = searchSvc.SearchByMultipleTags(req.Tags)
		} else {
			return nil, fmt.Errorf("tags required for tag search")
		}

	case "wikilink":
		if len(req.Wikilinks) == 1 {
			result, err = searchSvc.SearchByWikilink(req.Wikilinks[0])
		} else if len(req.Wikilinks) > 1 {
			result, err = searchSvc.SearchByMultipleWikilinks(req.Wikilinks)
		} else {
			return nil, fmt.Errorf("wikilinks required for wikilink search")
		}

	case "fuzzy":
		result, err = searchSvc.FuzzySearch(req.Query, 2)

	case "phrase":
		result, err = searchSvc.PhraseSearch(req.Query)

	case "prefix":
		result, err = searchSvc.PrefixSearch(req.Query)

	case "title":
		result, err = searchSvc.SearchByTitleOnly(req.Query)

	default: // "text" or empty
		if req.TitleOnly {
			result, err = searchSvc.SearchByTitleOnly(req.Query)
		} else {
			result, err = searchSvc.SearchByText(req.Query)
		}
	}

	if err != nil {
		return nil, err
	}

	// Convert to response format
	return s.convertSearchResult(result), nil
}

// convertSearchResult converts bleve search results to API response
func (s *Server) convertSearchResult(result *bleve.SearchResult) *SearchResponse {
	if result == nil {
		return &SearchResponse{
			Total:   0,
			Results: []SearchResult{},
			Took:    "0s",
		}
	}

	results := make([]SearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		sr := SearchResult{
			ID:    hit.ID,
			Score: hit.Score,
		}

		// Add fields
		if len(hit.Fields) > 0 {
			sr.Fields = hit.Fields
		}

		// Add fragments
		if len(hit.Fragments) > 0 {
			sr.Fragments = hit.Fragments
		}

		results = append(results, sr)
	}

	return &SearchResponse{
		Total:   result.Total,
		Results: results,
		Took:    result.Took.String(),
	}
}

// extractVaultID extracts vault ID from URL path
func (s *Server) extractVaultID(urlPath, prefix string) string {
	path := strings.TrimPrefix(urlPath, prefix)
	path = strings.TrimSuffix(path, "/")
	return path
}
