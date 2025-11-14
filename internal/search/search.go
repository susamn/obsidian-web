package search

import (
	"fmt"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// SearchByText performs full-text search across all indexed content
func SearchByText(index bleve.Index, queryStr string) (*bleve.SearchResult, error) {
	q := bleve.NewMatchQuery(queryStr)
	search := bleve.NewSearchRequest(q)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByTag searches for documents with a specific tag
func SearchByTag(index bleve.Index, tag string) (*bleve.SearchResult, error) {
	q := bleve.NewMatchQuery(tag)
	q.SetField("tags")
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByMultipleTags searches for documents matching all specified tags (AND)
func SearchByMultipleTags(index bleve.Index, tags []string) (*bleve.SearchResult, error) {
	queries := make([]query.Query, len(tags))
	for i, tag := range tags {
		q := bleve.NewMatchQuery(tag)
		q.SetField("tags")
		queries[i] = q
	}

	// Use conjunction (AND) for all tags
	q := bleve.NewConjunctionQuery(queries...)
	search := bleve.NewSearchRequest(q)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByWikilink searches for documents that contain a specific wikilink
func SearchByWikilink(index bleve.Index, wikilink string) (*bleve.SearchResult, error) {
	query := bleve.NewMatchQuery(wikilink)
	query.SetField("wikilinks")
	search := bleve.NewSearchRequest(query)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByBacklinks finds all documents that link to a specific note
// This is an alias for SearchByWikilink with clearer semantics
func SearchByBacklinks(index bleve.Index, noteName string) (*bleve.SearchResult, error) {
	return SearchByWikilink(index, noteName)
}

// SearchByTagsOR searches for documents matching ANY of the specified tags (OR logic)
func SearchByTagsOR(index bleve.Index, tags []string) (*bleve.SearchResult, error) {
	queries := make([]query.Query, len(tags))
	for i, tag := range tags {
		q := bleve.NewMatchQuery(tag)
		q.SetField("tags")
		queries[i] = q
	}

	// Use disjunction (OR) for tags
	query := bleve.NewDisjunctionQuery(queries...)
	search := bleve.NewSearchRequest(query)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByMultipleWikilinks searches for documents containing all specified wikilinks (AND)
func SearchByMultipleWikilinks(index bleve.Index, wikilinks []string) (*bleve.SearchResult, error) {
	queries := make([]query.Query, len(wikilinks))
	for i, wikilink := range wikilinks {
		q := bleve.NewMatchQuery(wikilink)
		q.SetField("wikilinks")
		queries[i] = q
	}

	// Use conjunction (AND) for all wikilinks
	query := bleve.NewConjunctionQuery(queries...)
	search := bleve.NewSearchRequest(query)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByWikilinksOR searches for documents containing ANY of the specified wikilinks (OR)
func SearchByWikilinksOR(index bleve.Index, wikilinks []string) (*bleve.SearchResult, error) {
	queries := make([]query.Query, len(wikilinks))
	for i, wikilink := range wikilinks {
		q := bleve.NewMatchQuery(wikilink)
		q.SetField("wikilinks")
		queries[i] = q
	}

	// Use disjunction (OR) for wikilinks
	query := bleve.NewDisjunctionQuery(queries...)
	search := bleve.NewSearchRequest(query)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchByTitleOnly searches only in the title field
func SearchByTitleOnly(index bleve.Index, queryStr string) (*bleve.SearchResult, error) {
	query := bleve.NewMatchQuery(queryStr)
	query.SetField("title")
	search := bleve.NewSearchRequest(query)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// FuzzySearch performs fuzzy text search (allows typos/misspellings)
func FuzzySearch(index bleve.Index, queryStr string, fuzziness int) (*bleve.SearchResult, error) {
	query := bleve.NewFuzzyQuery(queryStr)
	query.Fuzziness = fuzziness // 0 = exact, 1 = 1 character difference, 2 = 2 characters
	search := bleve.NewSearchRequest(query)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// PhraseSearch searches for an exact phrase
func PhraseSearch(index bleve.Index, phrase string) (*bleve.SearchResult, error) {
	query := bleve.NewMatchPhraseQuery(phrase)
	search := bleve.NewSearchRequest(query)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// PrefixSearch searches for terms starting with a prefix
func PrefixSearch(index bleve.Index, prefix string) (*bleve.SearchResult, error) {
	query := bleve.NewPrefixQuery(prefix)
	search := bleve.NewSearchRequest(query)
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// AdvancedSearch performs combined text and tag search
func AdvancedSearch(index bleve.Index, text string, tags []string) (*bleve.SearchResult, error) {
	queries := []query.Query{}

	// Add text query if provided
	if text != "" {
		queries = append(queries, bleve.NewMatchQuery(text))
	}

	// Add tag queries
	for _, tag := range tags {
		q := bleve.NewMatchQuery(tag)
		q.SetField("tags")
		queries = append(queries, q)
	}

	query := bleve.NewConjunctionQuery(queries...)
	search := bleve.NewSearchRequest(query)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// SearchCombined performs a comprehensive search with text, tags, and wikilinks
// All conditions are combined with AND logic
func SearchCombined(index bleve.Index, text string, tags []string, wikilinks []string) (*bleve.SearchResult, error) {
	queries := []query.Query{}

	// Add text query if provided
	if text != "" {
		queries = append(queries, bleve.NewMatchQuery(text))
	}

	// Add tag queries
	for _, tag := range tags {
		q := bleve.NewMatchQuery(tag)
		q.SetField("tags")
		queries = append(queries, q)
	}

	// Add wikilink queries
	for _, wikilink := range wikilinks {
		q := bleve.NewMatchQuery(wikilink)
		q.SetField("wikilinks")
		queries = append(queries, q)
	}

	if len(queries) == 0 {
		// No queries provided, return empty result
		return &bleve.SearchResult{}, nil
	}

	query := bleve.NewConjunctionQuery(queries...)
	search := bleve.NewSearchRequest(query)
	search.Highlight = bleve.NewHighlight()
	search.Fields = []string{"title", "path", "tags", "wikilinks"}
	search.Size = 20

	return index.Search(search)
}

// PrintResults displays search results in a formatted way
func PrintResults(results *bleve.SearchResult) {
	fmt.Printf("Total matches: %d (showing %d)\n", results.Total, len(results.Hits))
	for i, hit := range results.Hits {
		fmt.Printf("%d. %s (score: %.4f)\n", i+1, hit.ID, hit.Score)
		if title, ok := hit.Fields["title"].(string); ok {
			fmt.Printf("   Title: %s\n", title)
		}
		if tags, ok := hit.Fields["tags"].([]interface{}); ok {
			tagStrs := make([]string, len(tags))
			for i, t := range tags {
				tagStrs[i] = t.(string)
			}
			fmt.Printf("   Tags: %v\n", tagStrs)
		}
		if wikilinks, ok := hit.Fields["wikilinks"].([]interface{}); ok {
			linkStrs := make([]string, len(wikilinks))
			for i, l := range wikilinks {
				linkStrs[i] = l.(string)
			}
			fmt.Printf("   Wikilinks: %v\n", linkStrs)
		}
		fmt.Println()
	}
}
