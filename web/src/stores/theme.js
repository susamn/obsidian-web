import { defineStore } from 'pinia'

export const useThemeStore = defineStore('theme', {
  state: () => ({
    currentTheme: 'dracula', // Default theme
  }),
  actions: {
    setTheme(themeId) {
      this.currentTheme = themeId
      localStorage.setItem('theme', themeId)
      this.applyThemeToDocument(themeId)
    },
    loadThemeFromLocalStorage() {
      const savedTheme = localStorage.getItem('theme')
      if (savedTheme) {
        this.currentTheme = savedTheme
      }
      this.applyThemeToDocument(this.currentTheme)
    },
    applyThemeToDocument(themeId) {
      // Remove existing theme link
      const existingLink = document.getElementById('theme-link')
      if (existingLink) {
        existingLink.remove()
      }

      // Add new theme link
      const link = document.createElement('link')
      link.id = 'theme-link'
      link.rel = 'stylesheet'
      link.href = `/assets/themes/ow-theme-${themeId}.css`
      document.head.appendChild(link)
    },
  },
})
