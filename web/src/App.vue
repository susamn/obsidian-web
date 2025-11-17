<template>
  <div id="app" class="dark">
    <router-view />
  </div>
</template>

<script>
export default {
  name: 'App',
};
</script>

<style>
#app {
  font-family: Avenir, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-align: center;
  color: #2c3e50;
}

#app.dark {
  background-color: #1a1a1a;
  color: #f0f0f0;
  min-height: 100vh;
}
</style>
