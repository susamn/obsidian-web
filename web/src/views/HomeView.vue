<template>
  <div class="home-view">
    <header class="header">
      <h1 class="title">Obsidian Web</h1>
      <p class="subtitle">Your notes, on the web.</p>
    </header>

    <main class="main-content">
      <div class="vaults-section">
        <h2 class="section-title">Your Vaults</h2>
        <ul v-if="vaults.length" class="vault-list">
          <li
            v-for="vault in vaults"
            :key="vault.id"
            class="vault-item"
            @click="openVault(vault.id)"
          >
            <span class="vault-name">{{ vault.name }}</span>
            <span class="vault-status" :class="vault.status.toLowerCase()">{{ vault.status }}</span>
          </li>
        </ul>
        <p v-else class="no-vaults">No vaults found. Create one below.</p>
      </div>

      <div class="create-vault-section">
        <h2 class="section-title">Create New Vault</h2>
        <form @submit.prevent="createVault" class="create-vault-form">
          <input
            type="text"
            v-model="newVault.name"
            placeholder="Vault Name"
            class="form-input"
            required
          />
          <input
            type="text"
            v-model="newVault.path"
            placeholder="Vault Path"
            class="form-input"
            required
          />
          <button type="submit" class="form-button">Create Vault</button>
        </form>
      </div>
    </main>
  </div>
</template>

<script>
import axios from 'axios'

export default {
  name: 'HomeView',
  data() {
    return {
      vaults: [],
      newVault: {
        name: '',
        path: '',
      },
    }
  },
  async created() {
    await this.fetchVaults()
  },
  methods: {
    async fetchVaults() {
      try {
        const response = await axios.get('/api/v1/vaults')
        this.vaults = response.data.data.vaults
      } catch (error) {
        console.error('Error fetching vaults:', error)
      }
    },
    openVault(vaultId) {
      this.$router.push({ name: 'vault', params: { id: vaultId } })
    },
    async createVault() {
      // This is a placeholder. The backend API for creating vaults is not yet implemented.
      console.log('Creating vault:', this.newVault)
      alert('Creating vaults is not yet implemented.')
    },
  },
}
</script>

<style scoped>
.home-view {
  padding: 2rem;
  max-width: 800px;
  margin: 0 auto;
}

.header {
  text-align: center;
  margin-bottom: 3rem;
}

.title {
  font-size: 3rem;
  font-weight: bold;
  color: var(--primary-color);
}

.subtitle {
  font-size: 1.2rem;
  color: var(--text-color);
}

.main-content {
  display: grid;
  grid-template-columns: 1fr;
  gap: 3rem;
}

.section-title {
  font-size: 1.5rem;
  font-weight: bold;
  margin-bottom: 1rem;
  color: var(--primary-color);
}

.vault-list {
  list-style: none;
  padding: 0;
}

.vault-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  background-color: var(--background-color-light);
  border-radius: 8px;
  margin-bottom: 1rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.vault-item:hover {
  background-color: var(--background-color);
}

.vault-name {
  font-weight: bold;
}

.vault-status {
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.8rem;
  text-transform: uppercase;
}

.vault-status.active {
  background-color: #28a745;
  color: white;
}

.vault-status.inactive {
  background-color: #dc3545;
  color: white;
}

.no-vaults {
  color: var(--text-color);
}

.create-vault-form {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-input {
  padding: 0.8rem;
  border: 1px solid var(--border-color);
  background-color: var(--background-color-light);
  color: var(--text-color);
  border-radius: 4px;
}

.form-button {
  padding: 0.8rem;
  background-color: var(--primary-color);
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-weight: bold;
  transition: background-color 0.2s;
}

.form-button:hover {
  background-color: var(--primary-color);
  filter: brightness(1.2);
}
</style>
