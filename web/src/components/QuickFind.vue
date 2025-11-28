<template>
  <div class="quick-find">
    <div class="input-wrapper">
      <i class="fas fa-filter search-icon" />
      <input
        ref="inputRef"
        v-model="query"
        type="text"
        placeholder="Filter files..."
        class="quick-find-input"
        @keydown.down.prevent="navigateDown"
        @keydown.up.prevent="navigateUp"
        @keydown.enter.prevent="selectCurrent"
        @keydown.esc="clear"
      >
      <button
        v-if="query"
        class="clear-button"
        @click="clear"
      >
        <i class="fas fa-times" />
      </button>
    </div>

    <!-- Results list (inline) -->
    <div
      v-if="showResults && results.length > 0"
      class="results-list"
    >
      <div
        v-for="(node, index) in results"
        :key="node.metadata.id"
        class="result-item"
        :class="{ active: index === activeIndex }"
        @click="selectResult(node)"
        @mouseenter="activeIndex = index"
      >
        <i
          :class="getFileIcon(node.metadata)"
          class="item-icon"
        />
        <div class="item-details">
          <span class="item-name">{{ node.metadata.name }}</span>
          <span class="item-path">{{ node.metadata.path }}</span>
        </div>
      </div>
    </div>

    <div
      v-else-if="showResults && query && results.length === 0"
      class="no-results"
    >
      No files found
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { usePersistentTreeStore } from '../stores/persistentTreeStore'

const props = defineProps({
  vaultId: {
    type: String,
    required: true,
  },
})

const emit = defineEmits(['select'])

const store = usePersistentTreeStore()
const query = ref('')
const activeIndex = ref(0)
const inputRef = ref(null)

// Get all files from the store
const allFiles = computed(() => {
  const pathMap = store.pathIndex.get(props.vaultId)
  if (!pathMap) return []

  // Convert map values to array and filter only files (not directories)
  return Array.from(pathMap.values()).filter((node) => !node.metadata.is_directory)
})

// Filter files based on query
const results = computed(() => {
  if (!query.value || query.value.trim() === '') return []

  const q = query.value.toLowerCase().trim()

  // Simple limit to avoid performance issues with large vaults
  const MAX_RESULTS = 20

  return allFiles.value
    .filter((node) => {
      const name = node.metadata.name.toLowerCase()
      const path = node.metadata.path.toLowerCase()
      return name.includes(q) || path.includes(q)
    })
    .slice(0, MAX_RESULTS)
})

const showResults = computed(() => query.value.length > 0)

// Reset active index when results change
watch(results, () => {
  activeIndex.value = 0
})

const navigateDown = () => {
  if (activeIndex.value < results.value.length - 1) {
    activeIndex.value++
    scrollToActive()
  }
}

const navigateUp = () => {
  if (activeIndex.value > 0) {
    activeIndex.value--
    scrollToActive()
  }
}

const selectCurrent = () => {
  if (results.value.length > 0) {
    selectResult(results.value[activeIndex.value])
  }
}

const selectResult = (node) => {
  emit('select', node)
  clear()
}

const clear = () => {
  query.value = ''
  activeIndex.value = 0
  inputRef.value?.blur()
}

const scrollToActive = () => {
  // Optional: implement scrolling if list is scrollable
  const el = document.querySelector('.result-item.active')
  if (el) {
    el.scrollIntoView({ block: 'nearest' })
  }
}

/**
 * Get appropriate icon class based on file type
 */
const getFileIcon = (metadata) => {
  if (metadata.is_markdown) {
    return 'fas fa-file-alt'
  }
  const extension = metadata.name.split('.').pop().toLowerCase()
  const iconMap = {
    canvas: 'fas fa-project-diagram',
    png: 'fas fa-file-image',
    jpg: 'fas fa-file-image',
    pdf: 'fas fa-file-pdf',
    // Add more as needed, defaulting to generic file
  }
  return iconMap[extension] || 'fas fa-file'
}
</script>

<style scoped>
.quick-find {
  margin-bottom: 0.5rem;
  position: relative;
}

.input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.search-icon {
  position: absolute;
  left: 0.75rem;
  color: var(--text-color-secondary);
  font-size: 0.8rem;
  pointer-events: none;
}

.quick-find-input {
  width: 100%;
  padding: 0.4rem 2rem 0.4rem 2rem;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background-color: var(--background-color);
  color: var(--text-color);
  font-size: 0.85rem;
  outline: none;
  transition:
    border-color 0.2s,
    box-shadow 0.2s;
}

.quick-find-input:focus {
  border-color: var(--primary-color);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.clear-button {
  position: absolute;
  right: 0.5rem;
  background: none;
  border: none;
  color: var(--text-color-secondary);
  cursor: pointer;
  padding: 0.25rem;
  font-size: 0.8rem;
  display: flex;
  align-items: center;
  justify-content: center;
}

.clear-button:hover {
  color: var(--text-color);
}

.results-list {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background-color: var(--background-color);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  margin-top: 4px;
  max-height: 300px;
  overflow-y: auto;
  z-index: 50;
  box-shadow:
    0 4px 6px -1px rgba(0, 0, 0, 0.1),
    0 2px 4px -1px rgba(0, 0, 0, 0.06);
}

.result-item {
  display: flex;
  align-items: center;
  padding: 0.5rem;
  cursor: pointer;
  border-bottom: 1px solid var(--border-color);
  transition: background-color 0.1s;
}

.result-item:last-child {
  border-bottom: none;
}

.result-item:hover,
.result-item.active {
  background-color: color-mix(in srgb, var(--primary-color), transparent 85%);
}

.result-item.active {
  border-left: 3px solid var(--primary-color);
  padding-left: calc(0.5rem - 3px);
}

.result-item.active .item-name {
  color: var(--primary-color);
}

.item-icon {
  margin-right: 0.75rem;
  color: var(--text-color-secondary);
  width: 16px;
  text-align: center;
}

.item-details {
  display: flex;
  flex-direction: column;
  min-width: 0; /* Enable truncation */
}

.item-name {
  font-weight: 500;
  font-size: 0.9rem;
  color: var(--text-color);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.item-path {
  font-size: 0.75rem;
  color: var(--text-color-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.no-results {
  padding: 0.5rem;
  text-align: center;
  color: var(--text-color-secondary);
  font-size: 0.85rem;
  background-color: var(--background-color);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  margin-top: 4px;
  position: absolute;
  width: 100%;
  z-index: 50;
}
</style>
