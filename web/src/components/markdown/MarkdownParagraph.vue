<template>
  <p class="md-paragraph">
    <InlineRenderer :tokens="node.content" @wikilink-click="$emit('wikilink-click', $event)" />
  </p>
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
.md-paragraph {
  margin: 0.8em 0;
  line-height: 1.8;
}
</style>
