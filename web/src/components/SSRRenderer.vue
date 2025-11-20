<template>
  <div class="ssr-renderer">
    <div class="ssr-placeholder">
      <i class="fas fa-server"></i>
      <p>Server-side rendering is not available yet</p>
      <p class="ssr-info">This file will be rendered by the server</p>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';

const props = defineProps({
  content: {
    type: String,
    default: '',
  },
});

const emit = defineEmits(['update:markdownResult']);

// Empty for now - will be implemented later with server-side rendering
const markdownResult = ref({
  html: '',
  tags: [],
  frontmatter: {},
  headings: [],
  wikilinks: [],
  stats: { words: 0, chars: 0, readingTime: 0 }
});

onMounted(() => {
  // Emit empty markdown result on mount
  emit('update:markdownResult', markdownResult.value);
});
</script>

<style scoped>
.ssr-renderer {
  flex: 1;
  overflow-y: auto;
  padding: 0;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.ssr-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 3rem;
  color: var(--text-color-secondary);
}

.ssr-placeholder i {
  font-size: 3rem;
  margin-bottom: 1rem;
  opacity: 0.5;
}

.ssr-placeholder p {
  margin: 0.5rem 0;
}

.ssr-info {
  font-size: 0.9rem;
  font-style: italic;
  margin-top: 1rem;
}
</style>
