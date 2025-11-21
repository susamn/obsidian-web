<template>
  <hr class="md-hr" />
</template>

<script setup>
defineProps({
  node: {
    type: Object,
    required: true
  }
});
</script>

<style scoped>
.md-hr {
  border: none;
  border-top: 2px solid var(--border-color, #e2e8f0);
  margin: 2em 0;
}
</style>
