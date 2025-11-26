<template>
  <div class="search-panel">
    <div class="search-header">
      <button class="back-button" @click="onClose" title="Back to file browser">
        <i class="fas fa-folder"></i>
      </button>
      <div class="search-input-wrapper">
        <i class="fas fa-search search-icon"></i>
        <input
          ref="searchInput"
          v-model="searchQuery"
          type="text"
          class="search-input"
          :placeholder="getPlaceholder()"
          @keyup.enter="handleSearch"
          @input="handleInput"
        />
        <button v-if="searchQuery" class="clear-button" @click="clearQuery" title="Clear search">
          <i class="fas fa-times"></i>
        </button>
      </div>
    </div>

    <div class="search-filters">
      <select v-model="selectedSearchType" class="search-type-select">
        <option value="text">Text</option>
        <option value="title">Title Only</option>
        <option value="tag">Tag</option>
        <option value="wikilink">Wikilink</option>
        <option value="fuzzy">Fuzzy</option>
        <option value="phrase">Phrase</option>
        <option value="prefix">Prefix</option>
      </select>
      <button class="search-button" @click="handleSearch" :disabled="!searchQuery.trim()">
        Search
      </button>
    </div>

    <div class="search-info" v-if="searchStore.total > 0">
      <span class="result-count">{{ searchStore.total }} results</span>
      <span class="search-time">({{ searchStore.took }})</span>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useSearchStore } from '../stores/searchStore'

const props = defineProps({
  vaultId: {
    type: String,
    required: true,
  },
})

const emit = defineEmits(['close', 'search'])

const searchStore = useSearchStore()
const searchQuery = ref('')
const selectedSearchType = ref('text')
const searchInput = ref(null)

const getPlaceholder = () => {
  switch (selectedSearchType.value) {
    case 'tag':
      return 'Search by tag (e.g., #important)...'
    case 'wikilink':
      return 'Search by wikilink (e.g., [[Note]])...'
    case 'fuzzy':
      return 'Fuzzy search...'
    case 'phrase':
      return 'Search exact phrase...'
    case 'prefix':
      return 'Search by prefix...'
    case 'title':
      return 'Search in titles only...'
    default:
      return 'Search notes...'
  }
}

const handleSearch = async () => {
  if (!searchQuery.value.trim()) return

  try {
    const query = searchQuery.value.trim()

    switch (selectedSearchType.value) {
      case 'tag':
        // Parse tags from input (support both #tag and tag formats)
        const tags = query.split(/[,\s]+/).map((t) => t.replace(/^#/, ''))
        await searchStore.searchByTags(props.vaultId, tags)
        break
      case 'wikilink':
        // Parse wikilinks from input (support both [[link]] and link formats)
        const wikilinks = query.split(/[,\s]+/).map((w) => w.replace(/^\[\[|\]\]$/g, ''))
        await searchStore.searchByWikilinks(props.vaultId, wikilinks)
        break
      case 'fuzzy':
        await searchStore.fuzzySearch(props.vaultId, query)
        break
      case 'phrase':
        await searchStore.phraseSearch(props.vaultId, query)
        break
      case 'prefix':
        await searchStore.prefixSearch(props.vaultId, query)
        break
      case 'title':
        await searchStore.searchByText(props.vaultId, query, true)
        break
      default:
        await searchStore.searchByText(props.vaultId, query, false)
    }

    emit('search')
  } catch (error) {
    console.error('[SearchPanel] Search error:', error)
  }
}

const handleInput = () => {
  // Optional: Implement debounced search here if desired
}

const clearQuery = () => {
  searchQuery.value = ''
  searchStore.clearSearch()
  searchInput.value?.focus()
}

const onClose = () => {
  searchStore.clearSearch()
  searchQuery.value = ''
  emit('close')
}

onMounted(() => {
  // Auto-focus search input when panel opens
  searchInput.value?.focus()
})
</script>

<style scoped>
.search-panel {
  padding: 0.5rem 0;
}

.search-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.back-button {
  background: none;
  border: none;
  color: var(--text-color);
  cursor: pointer;
  padding: 0.5rem;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s;
  font-size: 1rem;
}

.back-button:hover {
  background-color: var(--background-color);
}

.search-input-wrapper {
  flex: 1;
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 0.75rem;
  color: var(--text-color-secondary);
  font-size: 0.875rem;
}

.search-input {
  width: 100%;
  padding: 0.5rem 2rem 0.5rem 2rem;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background-color: var(--background-color);
  color: var(--text-color);
  font-size: 0.875rem;
  outline: none;
  transition: border-color 0.2s;
}

.search-input:focus {
  border-color: var(--primary-color);
}

.clear-button {
  position: absolute;
  right: 0.5rem;
  background: none;
  border: none;
  color: var(--text-color-secondary);
  cursor: pointer;
  padding: 0.25rem;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s;
}

.clear-button:hover {
  background-color: var(--background-color-light);
}

.search-filters {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.search-type-select {
  flex: 1;
  padding: 0.5rem;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background-color: var(--background-color);
  color: var(--text-color);
  font-size: 0.875rem;
  outline: none;
  cursor: pointer;
}

.search-button {
  padding: 0.5rem 1rem;
  background-color: var(--primary-color);
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 500;
  transition: opacity 0.2s;
}

.search-button:hover:not(:disabled) {
  opacity: 0.9;
}

.search-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.search-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
  color: var(--text-color-secondary);
  padding: 0.25rem 0;
  border-bottom: 1px solid var(--border-color);
  margin-bottom: 0.5rem;
}

.result-count {
  font-weight: 500;
}

.search-time {
  opacity: 0.7;
}
</style>
