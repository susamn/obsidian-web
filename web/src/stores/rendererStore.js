import { defineStore } from 'pinia';

export const useRendererStore = defineStore('renderer', {
  state: () => ({
    rendererType: 'structured', // 'browser', 'ssr', or 'structured'
  }),

  getters: {
    currentRenderer: (state) => state.rendererType,
    isBrowserRenderer: (state) => state.rendererType === 'browser',
    isSSRRenderer: (state) => state.rendererType === 'ssr',
    isStructuredRenderer: (state) => state.rendererType === 'structured',
  },

  actions: {
    setRenderer(type) {
      if (!['browser', 'ssr', 'structured'].includes(type)) {
        console.warn(`Invalid renderer type: ${type}. Must be 'browser', 'ssr', or 'structured'`);
        return;
      }
      this.rendererType = type;
      // Persist to localStorage
      localStorage.setItem('rendererType', type);
    },

    loadRendererFromLocalStorage() {
      const saved = localStorage.getItem('rendererType');
      if (saved && ['browser', 'ssr', 'structured'].includes(saved)) {
        this.rendererType = saved;
      }
    },
  },
});
