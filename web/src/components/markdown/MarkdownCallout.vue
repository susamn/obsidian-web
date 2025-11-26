<template>
  <div :class="['md-callout', calloutClass]">
    <div class="md-callout-header">
      <span class="md-callout-icon">{{ calloutIcon }}</span>
      <span class="md-callout-title">{{ node.title }}</span>
    </div>
    <div class="md-callout-content">
      <InlineRenderer :tokens="node.content" @wikilink-click="$emit('wikilink-click', $event)" />
    </div>
  </div>
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

const CALLOUT_TYPES = {
  note: { icon: '📝', class: 'md-callout-note' },
  abstract: { icon: '📋', class: 'md-callout-abstract' },
  summary: { icon: '📋', class: 'md-callout-summary' },
  tldr: { icon: '📋', class: 'md-callout-tldr' },
  info: { icon: 'ℹ️', class: 'md-callout-info' },
  tip: { icon: '💡', class: 'md-callout-tip' },
  hint: { icon: '💡', class: 'md-callout-hint' },
  important: { icon: '❗', class: 'md-callout-important' },
  warning: { icon: '⚠️', class: 'md-callout-warning' },
  caution: { icon: '⚠️', class: 'md-callout-caution' },
  attention: { icon: '⚠️', class: 'md-callout-attention' },
  danger: { icon: '🔥', class: 'md-callout-danger' },
  error: { icon: '❌', class: 'md-callout-error' },
  failure: { icon: '❌', class: 'md-callout-failure' },
  bug: { icon: '🐛', class: 'md-callout-bug' },
  example: { icon: '📊', class: 'md-callout-example' },
  quote: { icon: '💬', class: 'md-callout-quote' },
}

const calloutConfig = computed(() => {
  return CALLOUT_TYPES[props.node.calloutType] || CALLOUT_TYPES.note
})

const calloutIcon = computed(() => calloutConfig.value.icon)
const calloutClass = computed(() => calloutConfig.value.class)
</script>

<style scoped>
/* Callout Base */
.md-callout {
  border-radius: 6px;
  margin: 1.2em 0;
  padding: 0;
  border: 1px solid;
  overflow: hidden;
  background-color: var(--background-color, #ffffff);
}

.md-callout-header {
  display: flex;
  align-items: center;
  gap: 0.5em;
  padding: 0.75em 1em;
  font-weight: 600;
  border-bottom: 1px solid;
}

.md-callout-icon {
  font-size: 1.2em;
  line-height: 1;
  display: flex;
  align-items: center;
}

.md-callout-title {
  font-size: 0.95em;
  line-height: 1.3;
}

.md-callout-content {
  padding: 1em;
  line-height: 1.6;
}

/* Callout Types */
.md-callout-note,
.md-callout-abstract,
.md-callout-summary,
.md-callout-tldr {
  border-color: rgba(59, 130, 246, 0.3);
  background-color: rgba(59, 130, 246, 0.05);
}

.md-callout-note .md-callout-header,
.md-callout-abstract .md-callout-header,
.md-callout-summary .md-callout-header,
.md-callout-tldr .md-callout-header {
  background-color: rgba(59, 130, 246, 0.1);
  border-bottom-color: rgba(59, 130, 246, 0.2);
  color: #3b82f6;
}

.md-callout-info {
  border-color: rgba(14, 165, 233, 0.3);
  background-color: rgba(14, 165, 233, 0.05);
}

.md-callout-info .md-callout-header {
  background-color: rgba(14, 165, 233, 0.1);
  border-bottom-color: rgba(14, 165, 233, 0.2);
  color: #0ea5e9;
}

.md-callout-tip,
.md-callout-hint {
  border-color: rgba(16, 185, 129, 0.3);
  background-color: rgba(16, 185, 129, 0.05);
}

.md-callout-tip .md-callout-header,
.md-callout-hint .md-callout-header {
  background-color: rgba(16, 185, 129, 0.1);
  border-bottom-color: rgba(16, 185, 129, 0.2);
  color: #10b981;
}

.md-callout-important {
  border-color: rgba(168, 85, 247, 0.3);
  background-color: rgba(168, 85, 247, 0.05);
}

.md-callout-important .md-callout-header {
  background-color: rgba(168, 85, 247, 0.1);
  border-bottom-color: rgba(168, 85, 247, 0.2);
  color: #a855f7;
}

.md-callout-warning,
.md-callout-caution,
.md-callout-attention {
  border-color: rgba(251, 146, 60, 0.3);
  background-color: rgba(251, 146, 60, 0.05);
}

.md-callout-warning .md-callout-header,
.md-callout-caution .md-callout-header,
.md-callout-attention .md-callout-header {
  background-color: rgba(251, 146, 60, 0.1);
  border-bottom-color: rgba(251, 146, 60, 0.2);
  color: #fb923c;
}

.md-callout-danger,
.md-callout-error,
.md-callout-failure,
.md-callout-bug {
  border-color: rgba(239, 68, 68, 0.3);
  background-color: rgba(239, 68, 68, 0.05);
}

.md-callout-danger .md-callout-header,
.md-callout-error .md-callout-header,
.md-callout-failure .md-callout-header,
.md-callout-bug .md-callout-header {
  background-color: rgba(239, 68, 68, 0.1);
  border-bottom-color: rgba(239, 68, 68, 0.2);
  color: #ef4444;
}

.md-callout-example {
  border-color: rgba(139, 92, 246, 0.3);
  background-color: rgba(139, 92, 246, 0.05);
}

.md-callout-example .md-callout-header {
  background-color: rgba(139, 92, 246, 0.1);
  border-bottom-color: rgba(139, 92, 246, 0.2);
  color: #8b5cf6;
}

.md-callout-quote {
  border-color: rgba(148, 163, 184, 0.3);
  background-color: rgba(148, 163, 184, 0.05);
}

.md-callout-quote .md-callout-header {
  background-color: rgba(148, 163, 184, 0.1);
  border-bottom-color: rgba(148, 163, 184, 0.2);
  color: #94a3b8;
}
</style>
