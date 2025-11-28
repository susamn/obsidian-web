<template>
  <div
    class="md-latex"
    v-html="renderedHtml"
  />
</template>

<script setup>
import { computed } from 'vue'
import katex from 'katex'
import 'katex/dist/katex.min.css'

const props = defineProps({
  node: {
    type: Object,
    required: true,
  },
})

const renderedHtml = computed(() => {
  try {
    return katex.renderToString(props.node.content, {
      throwOnError: false,
      displayMode: true,
    })
  } catch (error) {
    console.error('KaTeX error:', error)
    return `<span class="latex-error">Error parsing LaTeX: ${error.message}</span>`
  }
})
</script>

<style scoped>
.md-latex {
  margin: 1em 0;
  text-align: center;
  overflow-x: auto;
  padding: 0.5em;
}

.latex-error {
  color: #ef4444;
  font-family: monospace;
  font-size: 0.9em;
}
</style>
