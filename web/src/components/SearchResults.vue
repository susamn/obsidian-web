<template>
  <div class="search-results">
    <div v-if="searchStore.loading" class="loading-state">
      <i class="fas fa-spinner fa-spin"></i>
      <span>Searching...</span>
    </div>

    <div v-else-if="searchStore.error" class="error-state">
      <i class="fas fa-exclamation-circle"></i>
      <span>{{ searchStore.error }}</span>
    </div>

    <div v-else-if="searchStore.results.length === 0 && searchStore.query" class="empty-state">
      <i class="fas fa-search"></i>
      <span>No results found</span>
    </div>

    <div v-else class="results-list">
      <div
        v-for="result in searchStore.results"
        :key="result.id"
        class="result-item"
        @click="handleResultClick(result)"
      >
        <div class="result-header">
          <i class="fas fa-file-alt result-icon"></i>
          <span class="result-title">{{ getFileName(result) }}</span>
          <span class="result-score" :title="`Relevance: ${result.score.toFixed(2)}`">
            {{ formatScore(result.score) }}
          </span>
        </div>

        <div v-if="result.fragments && Object.keys(result.fragments).length > 0" class="result-fragments">
          <div
            v-for="(fragments, field) in result.fragments"
            :key="field"
            class="fragment-group"
          >
            <div
              v-for="(fragment, index) in fragments"
              :key="index"
              class="fragment"
              v-html="sanitizeFragment(fragment)"
            ></div>
          </div>
        </div>

        <div v-if="result.fields" class="result-meta">
          <span v-if="result.fields.tags" class="meta-tags">
            <i class="fas fa-tag"></i>
            {{ Array.isArray(result.fields.tags) ? result.fields.tags.join(', ') : result.fields.tags }}
          </span>
          <span v-if="result.fields.path" class="meta-path">
            <i class="fas fa-folder"></i>
            {{ result.fields.path }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useSearchStore } from '../stores/searchStore';

const props = defineProps({
  vaultId: {
    type: String,
    required: true,
  },
});

const emit = defineEmits(['result-selected']);

const searchStore = useSearchStore();

const getFileName = (result) => {
  // Use the path from fields if available, otherwise fallback to ID
  const path = result.fields?.path || result.id;
  const parts = path.split('/');
  return parts[parts.length - 1] || path;
};

const formatScore = (score) => {
  // Format score as percentage
  const percentage = Math.round(score * 100);
  return `${percentage}%`;
};

const sanitizeFragment = (fragment) => {
  // The fragment HTML from backend contains <mark> tags for highlighting
  // We trust the backend to provide safe HTML, but we can add additional sanitization if needed
  return fragment;
};

const handleResultClick = (result) => {
  console.log('[SearchResults] Result clicked:', result);
  emit('result-selected', result);
};
</script>

<style scoped>
.search-results {
  height: 100%;
  overflow-y: auto;
}

.loading-state,
.error-state,
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  padding: 2rem 1rem;
  color: var(--text-color-secondary);
  font-size: 0.875rem;
}

.loading-state i,
.error-state i,
.empty-state i {
  font-size: 2rem;
  opacity: 0.5;
}

.error-state {
  color: #e06c75;
}

.results-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.result-item {
  padding: 0.75rem;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background-color: var(--background-color);
  cursor: pointer;
  transition: all 0.2s;
}

.result-item:hover {
  background-color: var(--background-color-light);
  border-color: var(--primary-color);
  transform: translateX(2px);
}

.result-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}

.result-icon {
  color: var(--primary-color);
  font-size: 0.875rem;
}

.result-title {
  flex: 1;
  font-weight: 500;
  color: var(--text-color);
  font-size: 0.875rem;
}

.result-score {
  font-size: 0.75rem;
  color: var(--text-color-secondary);
  background-color: var(--background-color-light);
  padding: 0.125rem 0.5rem;
  border-radius: 12px;
}

.result-fragments {
  margin: 0.5rem 0;
  font-size: 0.8rem;
  line-height: 1.5;
}

.fragment-group {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.fragment {
  color: var(--text-color-secondary);
  padding: 0.25rem 0;
}

.fragment :deep(mark) {
  background-color: rgba(255, 215, 0, 0.3);
  color: var(--text-color);
  font-weight: 500;
  padding: 0.125rem 0.25rem;
  border-radius: 2px;
}

.result-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-top: 0.5rem;
  padding-top: 0.5rem;
  border-top: 1px solid var(--border-color);
  font-size: 0.75rem;
}

.meta-tags,
.meta-path {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  color: var(--text-color-secondary);
}

.meta-tags i,
.meta-path i {
  font-size: 0.7rem;
}
</style>
