<template>
  <div class="vault-view">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h2 class="vault-name">{{ vaultName }}</h2>
      </div>
      <div class="file-tree">
        <p v-if="fileStore.loading">Loading file tree...</p>
        <p v-else-if="fileStore.error" class="text-red-500">Error: {{ fileStore.error }}</p>
        <FileTree
          v-else
          :nodes="fileStore.treeData"
          :vault-id="fileStore.vaultId"
          :expanded-nodes="expandedNodes"
          @toggle-expand="handleToggleExpand"
        />
      </div>
    </aside>
    <main class="main-content">
      <p>Main content will be here.</p>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue';
import { useRoute } from 'vue-router';
import { useFileStore } from '../stores/fileStore';
import FileTree from '../components/FileTree.vue';

const route = useRoute();
const fileStore = useFileStore();
const vaultName = ref('');
const expandedNodes = ref({});

const handleToggleExpand = async (node) => {
  if (node.metadata.is_directory) {
    if (expandedNodes.value[node.metadata.path]) {
      // Collapse
      delete expandedNodes.value[node.metadata.path];
    } else {
      // Expand
      expandedNodes.value[node.metadata.path] = true;
      // Fetch children if not already fetched
      if (!node.children || node.children.length === 0) {
        await fileStore.fetchChildren(fileStore.vaultId, node.metadata.path);
        // Assuming the API returns children for the given path,
        // we need to find the node in treeData and update its children.
        // This is a simplified approach; a more robust solution might involve
        // normalizing the tree data in the store.
        updateNodeChildren(fileStore.treeData, node.metadata.path, fileStore.childrenData);
      }
    }
  }
};

const updateNodeChildren = (nodes, targetPath, newChildren) => {
  for (let i = 0; i < nodes.length; i++) {
    if (nodes[i].metadata.path === targetPath) {
      nodes[i].children = newChildren;
      return true;
    }
    if (nodes[i].metadata.is_directory && nodes[i].children) {
      if (updateNodeChildren(nodes[i].children, targetPath, newChildren)) {
        return true;
      }
    }
  }
  return false;
};

// Watch for changes in the route params, specifically the 'id' for the vault
watch(() => route.params.id, (newId) => {
  if (newId) {
    fileStore.setVaultId(newId);
    vaultName.value = `Vault ${newId}`;
    fileStore.fetchTree(newId);
    expandedNodes.value = {}; // Reset expanded nodes when vault changes
  }
}, { immediate: true }); // Immediate: true to run the watcher on initial component mount

onMounted(() => {
  // Initial fetch if not already done by watcher (e.g., direct navigation)
  if (!fileStore.vaultId && route.params.id) {
    fileStore.setVaultId(route.params.id);
    vaultName.value = `Vault ${route.params.id}`;
    fileStore.fetchTree(route.params.id);
  }
});
</script>

<style scoped>
.vault-view {
  display: flex;
  height: 100vh;
}

.sidebar {
  width: 250px;
  background-color: var(--background-color-light);
  padding: 1rem;
  border-right: 1px solid var(--border-color);
}

.sidebar-header {
  margin-bottom: 1rem;
}

.vault-name {
  font-size: 1.2rem;
  font-weight: bold;
  color: var(--primary-color);
}

.main-content {
  flex-grow: 1;
  padding: 2rem;
}
</style>