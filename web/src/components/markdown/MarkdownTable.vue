<template>
  <table class="md-table">
    <thead v-if="headerRow">
      <tr>
        <th v-for="(cell, index) in headerRow.cells" :key="index" class="md-table-header">
          <InlineRenderer :tokens="cell" @wikilink-click="$emit('wikilink-click', $event)" />
        </th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="(row, rowIndex) in dataRows" :key="rowIndex">
        <td v-for="(cell, cellIndex) in row.cells" :key="cellIndex" class="md-table-cell">
          <InlineRenderer :tokens="cell" @wikilink-click="$emit('wikilink-click', $event)" />
        </td>
      </tr>
    </tbody>
  </table>
</template>

<script setup>
import { computed } from 'vue';
import InlineRenderer from './InlineRenderer.vue';

const props = defineProps({
  node: {
    type: Object,
    required: true
  }
});

defineEmits(['wikilink-click']);

const headerRow = computed(() => {
  return props.node.rows.find(row => row.type === 'header');
});

const dataRows = computed(() => {
  return props.node.rows.filter(row => row.type === 'row');
});
</script>

<style scoped>
.md-table {
  width: 100%;
  border-collapse: collapse;
  margin: 1em 0;
}

.md-table-header,
.md-table-cell {
  border: 1px solid var(--md-table-border, #e2e8f0);
  padding: 0.75em 1em;
  text-align: left;
}

.md-table-header {
  background-color: var(--md-table-header-bg, #f8fafc);
  font-weight: bold;
}
</style>
