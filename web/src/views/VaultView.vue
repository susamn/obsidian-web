<template>
  <div class="vault-view">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h2 class="vault-name">{{ vaultName }}</h2>
      </div>
      <div class="file-tree">
        <p>File tree will be here.</p>
      </div>
    </aside>
    <main class="main-content">
      <p>Main content will be here.</p>
    </main>
  </div>
</template>

<script>
export default {
  name: 'VaultView',
  props: {
    id: {
      type: String,
      required: true,
    },
  },
  data() {
    return {
      vaultName: '',
    };
  },
  created() {
    // Fetch vault details here
    this.vaultName = `Vault ${this.id}`;
  },
};
</script>

<style scoped>
.vault-view {
  display: flex;
  height: 100vh;
}

.sidebar {
  width: 250px;
  background-color: #2a2a2a;
  padding: 1rem;
  border-right: 1px solid #444;
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