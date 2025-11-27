<template>
  <component :is="headingTag" :id="node.id" class="md-heading">
    <InlineRenderer :tokens="node.content" @wikilink-click="$emit('wikilink-click', $event)" />
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

const headingTag = computed(() => `h${props.node.level}`)
</script>

<style scoped>
.md-heading {
  color: var(--md-heading-color, #1e293b);
  line-height: 1.3;
  font-weight: 600;
  margin-top: 1.5em;
  margin-bottom: 0.5em;
}

h1.md-heading {
  font-size: 2.15em;
  font-weight: 700;
}

h2.md-heading {
  font-size: 1.75em;
}

h3.md-heading {
  font-size: 1.4em;
}

h4.md-heading {
  font-size: 1.2em;
}

h5.md-heading {
  font-size: 1.05em;
}

h6.md-heading {
  font-size: 1em;
}
</style>
