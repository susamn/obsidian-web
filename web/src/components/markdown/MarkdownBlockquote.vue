<template>
  <blockquote class="md-blockquote">
    <InlineRenderer :tokens="node.content" @wikilink-click="$emit('wikilink-click', $event)" />
  </blockquote>
</template>

<script setup>
import InlineRenderer from './InlineRenderer.vue'

defineProps({
  node: {
    type: Object,
    required: true,
  },
})

defineEmits(['wikilink-click'])
</script>

<style scoped>
.md-blockquote {
  border-left: 3px solid var(--border-color, #e2e8f0);
  padding-left: 1em;
  margin: 1em 0;
  color: var(--text-color-secondary, #64748b);
  font-style: italic;
}
</style>
