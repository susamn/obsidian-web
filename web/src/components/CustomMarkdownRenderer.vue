<template>
  <div class="custom-markdown-renderer">
    <component
      :is="getNodeComponent(node)"
      v-for="(node, index) in nodes"
      :key="index"
      :node="node"
      @wikilink-click="handleWikilinkClick"
    />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import MarkdownHeading from './markdown/MarkdownHeading.vue'
import MarkdownParagraph from './markdown/MarkdownParagraph.vue'
import MarkdownCodeBlock from './markdown/MarkdownCodeBlock.vue'
import MarkdownBlockquote from './markdown/MarkdownBlockquote.vue'
import MarkdownCallout from './markdown/MarkdownCallout.vue'
import MarkdownList from './markdown/MarkdownList.vue'
import MarkdownTable from './markdown/MarkdownTable.vue'
import MarkdownHr from './markdown/MarkdownHr.vue'
import MarkdownLatex from './markdown/MarkdownLatex.vue'

const props = defineProps({
  nodes: {
    type: Array,
    required: true,
    default: () => [],
  },
})

const emit = defineEmits(['wikilink-click'])

const componentMap = {
  heading: MarkdownHeading,
  paragraph: MarkdownParagraph,
  code_block: MarkdownCodeBlock,
  blockquote: MarkdownBlockquote,
  callout: MarkdownCallout,
  ul: MarkdownList,
  ol: MarkdownList,
  table: MarkdownTable,
  hr: MarkdownHr,
  latex: MarkdownLatex,
}

function getNodeComponent(node) {
  return componentMap[node.type] || MarkdownParagraph
}

function handleWikilinkClick(event) {
  emit('wikilink-click', event)
}
</script>

<style scoped>
.custom-markdown-renderer {
  color: var(--text-color);
  line-height: 1.8;
}
</style>
