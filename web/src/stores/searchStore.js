import { defineStore } from 'pinia';
import { ref } from 'vue';

export const useSearchStore = defineStore('search', () => {
  const results = ref([]);
  const loading = ref(false);
  const error = ref(null);
  const query = ref('');
  const searchType = ref('text'); // text, tag, wikilink, fuzzy, phrase, prefix, title
  const total = ref(0);
  const took = ref('0s');

  /**
   * Execute search against the backend
   * @param {string} vaultId - The vault ID
   * @param {object} searchParams - Search parameters
   */
  const executeSearch = async (vaultId, searchParams) => {
    loading.value = true;
    error.value = null;

    try {
      const response = await fetch(`/api/v1/search/${vaultId}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(searchParams),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Search failed');
      }

      const responseData = await response.json();

      // Handle response that might be wrapped in a 'data' object
      const data = responseData.data || responseData;

      results.value = data.results || [];
      total.value = data.total || 0;
      took.value = data.took || '0s';

      return data;
    } catch (err) {
      error.value = err.message;
      results.value = [];
      total.value = 0;
      throw err;
    } finally {
      loading.value = false;
    }
  };

  /**
   * Search by text query
   */
  const searchByText = async (vaultId, searchQuery, titleOnly = false) => {
    query.value = searchQuery;
    searchType.value = titleOnly ? 'title' : 'text';

    return executeSearch(vaultId, {
      query: searchQuery,
      type: titleOnly ? 'title' : 'text',
      limit: 100,
    });
  };

  /**
   * Search by tags
   */
  const searchByTags = async (vaultId, tags) => {
    searchType.value = 'tag';

    return executeSearch(vaultId, {
      type: 'tag',
      tags: Array.isArray(tags) ? tags : [tags],
      limit: 100,
    });
  };

  /**
   * Search by wikilinks
   */
  const searchByWikilinks = async (vaultId, wikilinks) => {
    searchType.value = 'wikilink';

    return executeSearch(vaultId, {
      type: 'wikilink',
      wikilinks: Array.isArray(wikilinks) ? wikilinks : [wikilinks],
      limit: 100,
    });
  };

  /**
   * Fuzzy search
   */
  const fuzzySearch = async (vaultId, searchQuery) => {
    query.value = searchQuery;
    searchType.value = 'fuzzy';

    return executeSearch(vaultId, {
      query: searchQuery,
      type: 'fuzzy',
      limit: 100,
    });
  };

  /**
   * Phrase search
   */
  const phraseSearch = async (vaultId, phrase) => {
    query.value = phrase;
    searchType.value = 'phrase';

    return executeSearch(vaultId, {
      query: phrase,
      type: 'phrase',
      limit: 100,
    });
  };

  /**
   * Prefix search
   */
  const prefixSearch = async (vaultId, prefix) => {
    query.value = prefix;
    searchType.value = 'prefix';

    return executeSearch(vaultId, {
      query: prefix,
      type: 'prefix',
      limit: 100,
    });
  };

  /**
   * Clear search results
   */
  const clearSearch = () => {
    results.value = [];
    query.value = '';
    error.value = null;
    total.value = 0;
    took.value = '0s';
  };

  return {
    results,
    loading,
    error,
    query,
    searchType,
    total,
    took,
    executeSearch,
    searchByText,
    searchByTags,
    searchByWikilinks,
    fuzzySearch,
    phraseSearch,
    prefixSearch,
    clearSearch,
  };
});
