<template>
  <div class="settings-view">
    <button @click="goBack" class="back-button">← Back</button>
    <h1>Settings</h1>

    <!-- Theme Selection -->
    <div class="theme-selection">
      <h2>Theme</h2>
      <select v-model="selectedTheme" @change="setTheme" class="theme-select">
        <option v-for="theme in themes" :key="theme.id" :value="theme.id">
          {{ theme.name }}
        </option>
      </select>
    </div>

    <!-- Renderer Selection -->
    <div class="renderer-selection">
      <h2>Markdown Renderer</h2>
      <p class="renderer-description">Choose how markdown files are rendered</p>
      <select v-model="selectedRenderer" @change="setRenderer" class="renderer-select">
        <option value="browser">Browser Markdown Rendering</option>
        <option value="ssr">Server Side Rendering</option>
      </select>
      <p class="renderer-info">
        <span v-if="selectedRenderer === 'browser'" class="info-text">
          <i class="fas fa-check-circle"></i> Browser rendering provides real-time syntax highlighting and live preview
        </span>
        <span v-else class="info-text">
          <i class="fas fa-check-circle"></i> Server-side rendering will be rendered by the backend
        </span>
      </p>
    </div>
  </div>
</template>

<script>
import { useThemeStore } from '../stores/theme';
import { useRendererStore } from '../stores/rendererStore';

export default {
  name: 'SettingsView',
  setup() {
    const themeStore = useThemeStore();
    const rendererStore = useRendererStore();

    // Load saved renderer preference
    rendererStore.loadRendererFromLocalStorage();

    return {
      themeStore,
      rendererStore,
    };
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
    };
  },
  computed: {
    selectedTheme: {
      get() {
        return this.themeStore.currentTheme;
      },
      set(value) {
        this.themeStore.setTheme(value);
      },
    },
    selectedRenderer: {
      get() {
        return this.rendererStore.currentRenderer;
      },
      set(value) {
        this.rendererStore.setRenderer(value);
      },
    },
  },
  methods: {
    setTheme(event) {
      this.themeStore.setTheme(event.target.value);
    },
    setRenderer(event) {
      this.rendererStore.setRenderer(event.target.value);
    },
    goBack() {
      this.$router.push('/');
    },
  },
};
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

.renderer-selection {
  margin-top: 2.5rem;
  padding-top: 2rem;
  border-top: 1px solid var(--border-color);
}

.renderer-description {
  color: var(--text-color-secondary);
  font-size: 0.95rem;
  margin-bottom: 1rem;
}

.renderer-select {
  padding: 0.5rem;
  border: 1px solid var(--border-color);
  background-color: var(--background-color-light);
  color: var(--text-color);
  border-radius: 4px;
  margin-bottom: 1rem;
}

.renderer-info {
  padding: 1rem;
  background-color: var(--background-color-light);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  color: var(--text-color-secondary);
  font-size: 0.9rem;
}

.info-text {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.info-text i {
  color: #10b981;
  font-size: 1rem;
}
</style>
