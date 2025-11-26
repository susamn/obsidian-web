<template>
  <component :is="listTag" class="md-list">
    <li v-for="(item, index) in node.items" :key="index" class="md-list-item">
      <InlineRenderer :tokens="item.content" @wikilink-click="$emit('wikilink-click', $event)" />
    </li>
  </component>
</template>

<script setup>
import { computed } from 'vue'
import InlineRenderer from './InlineRenderer.vue'

const props = defineProps({
  node: {
    type: Object,
    required: true,
  },
})

defineEmits(['wikilink-click'])

const listTag = computed(() => props.node.type)
</script>

<style scoped>
.md-list {
  margin: 0.8em 0;
  padding-left: 2.2em;
}

.md-list-item {
  margin-bottom: 0.45em;
  line-height: 1.75;
}
</style>
