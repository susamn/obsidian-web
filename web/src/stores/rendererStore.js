import { defineStore } from 'pinia';

export const useRendererStore = defineStore('renderer', {
  state: () => ({
    rendererType: 'browser', // 'browser' or 'ssr'
  }),

  getters: {
    currentRenderer: (state) => state.rendererType,
    isBrowserRenderer: (state) => state.rendererType === 'browser',
    isSSRRenderer: (state) => state.rendererType === 'ssr',
  },

  actions: {
    setRenderer(type) {
      if (!['browser', 'ssr'].includes(type)) {
        console.warn(`Invalid renderer type: ${type}. Must be 'browser' or 'ssr'`);
        return;
      }
      this.rendererType = type;
      // Persist to localStorage
      localStorage.setItem('rendererType', type);
    },

    loadRendererFromLocalStorage() {
      const saved = localStorage.getItem('rendererType');
      if (saved && ['browser', 'ssr'].includes(saved)) {
        this.rendererType = saved;
      }
    },
  },
});
