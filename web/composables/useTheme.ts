import { ref } from 'vue'

export type ThemeMode = 'midnight' | 'graphite'

const currentTheme = ref<ThemeMode>('graphite')

export function useTheme() {
  function setTheme(theme: ThemeMode) {
    currentTheme.value = theme
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem('attackbox-appearance', theme)
        localStorage.setItem('attackbox-learning-theme', theme)
        document.documentElement.setAttribute('data-theme', theme)
      } catch {}
    }
  }

  function initTheme() {
    if (typeof window === 'undefined') return
    try {
      const saved = (localStorage.getItem('attackbox-appearance') || localStorage.getItem('attackbox-learning-theme')) as ThemeMode | null
      if (saved === 'graphite' || saved === 'midnight') {
        currentTheme.value = saved
      } else {
        currentTheme.value = 'graphite'
      }
      document.documentElement.setAttribute('data-theme', currentTheme.value)
    } catch {
      currentTheme.value = 'graphite'
    }
  }

  return {
    currentTheme,
    setTheme,
    initTheme,
  }
}
