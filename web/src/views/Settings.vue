<template>
  <div class="settings-view">
    <button class="back-button" @click="goBack">← Back</button>
    <h1>Settings</h1>

    <!-- Theme Selection -->
    <div class="theme-selection">
      <h2>Theme</h2>
      <select v-model="selectedTheme" class="theme-select" @change="setTheme">
        <option v-for="theme in themes" :key="theme.id" :value="theme.id">
          {{ theme.name }}
        </option>
      </select>
    </div>
  </div>
</template>

<script>
import { useThemeStore } from '../stores/theme'

export default {
  name: 'SettingsView',
  setup() {
    const themeStore = useThemeStore()

    return {
      themeStore,
    }
  },
  data() {
    return {
      themes: [
        { id: 'dracula', name: 'Dracula' },
        { id: 'catppuccin', name: 'Catppuccin' },
        { id: 'solarized-dark', name: 'Solarized Dark' },
        { id: 'solarized-light', name: 'Solarized Light' },
        { id: 'one-dark', name: 'One Dark' },
        { id: 'monokai', name: 'Monokai' },
        { id: 'gruvbox-dark', name: 'Gruvbox Dark' },
        { id: 'nord-inspired', name: 'Nord Inspired' },
        { id: 'tokyo-night', name: 'Tokyo Night' },
      ],
    }
  },
  computed: {
    selectedTheme: {
      get() {
        return this.themeStore.currentTheme
      },
      set(value) {
        this.themeStore.setTheme(value)
      },
    },
  },
  methods: {
    setTheme(event) {
      this.themeStore.setTheme(event.target.value)
    },
    goBack() {
      this.$router.push('/')
    },
  },
}
</script>

<style scoped>
.settings-view {
  padding: 2rem;
  max-width: 800px;
  margin: 0 auto;
  text-align: left;
}

.back-button {
  background: none;
  border: none;
  color: var(--primary-color);
  font-size: 1rem;
  cursor: pointer;
  margin-bottom: 1rem;
}

.theme-selection {
  margin-top: 2rem;
}

.theme-select {
  padding: 0.5rem;
  border: 1px solid var(--border-color);
  background-color: var(--background-color-light);
  color: var(--text-color);
  border-radius: 4px;
}
</style>
