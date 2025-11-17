<template>
  <div class="top-bar">
    <div class="logo">
      Obsidian Web
    </div>
    <div class="actions">
      <div class="progress-indicator">
        <!-- Placeholder for circular progress -->
        <div class="progress-circle"></div>
      </div>
      <div class="settings-icon" @click="goToSettings">
        ⚙️
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'TopBar',
  methods: {
    goToSettings() {
      this.$router.push({ name: 'settings' });
    },
  },
};
</script>

<style scoped>
.top-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.5rem 1rem;
  background-color: var(--background-color-light);
  border-bottom: 1px solid var(--border-color);
  color: var(--text-color);
}

.logo {
  font-weight: bold;
}

.actions {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.settings-icon {
  cursor: pointer;
  font-size: 1.5rem;
}

.progress-circle {
  width: 20px;
  height: 20px;
  border: 2px solid var(--primary-color);
  border-radius: 50%;
  border-top-color: transparent;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
