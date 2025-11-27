<template>
  <div class="md-code-block">
    <div v-if="node.language" class="code-language">
      {{ node.language }}
    </div>
    <pre><code>{{ node.content }}</code></pre>
  </div>
</template>

<script setup>
defineProps({
  node: {
    type: Object,
    required: true,
  },
})
</script>

<style scoped>
.md-code-block {
  position: relative;
  margin: 1.2em 0;
  background-color: var(--md-pre-bg, #1e293b);
  border: 1px solid var(--md-pre-border, #334155);
  border-radius: 6px;
  overflow: hidden;
}

.code-language {
  position: absolute;
  top: 0.5em;
  right: 0.75em;
  font-size: 0.75em;
  color: #94a3b8;
  text-transform: uppercase;
  font-weight: 600;
  letter-spacing: 0.05em;
}

.md-code-block pre {
  margin: 0;
  padding: 1.2em;
  overflow-x: auto;
}

.md-code-block code {
  color: var(--md-code-text, #e2e8f0);
  font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;
  font-size: 0.9em;
  line-height: 1.6;
  background: none;
}
</style>
