<template>
  <div id="app">
    <TopBar />
    <router-view />
  </div>
</template>

<script>
import TopBar from './components/TopBar.vue'
import { useThemeStore } from './stores/theme'

export default {
  name: 'App',
  components: {
    TopBar,
  },
  setup() {
    const themeStore = useThemeStore()
    themeStore.loadThemeFromLocalStorage()
  },
}
</script>

<style>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-align: center;
  height: 100vh;
  overflow: hidden;
  background-color: var(--background-color);
  color: var(--text-color);
  display: flex;
  flex-direction: column;
}
</style>
