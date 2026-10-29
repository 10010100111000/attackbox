export type ThemeMode = 'midnight' | 'graphite'

export function useTheme() {
  const currentTheme = useState<ThemeMode>('attackbox-theme', () => 'graphite')

  function setTheme(theme: ThemeMode) {
    currentTheme.value = theme
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem('attackbox-appearance', theme)
        document.documentElement.setAttribute('data-theme', theme)
      } catch {}
    }
  }

  function initTheme() {
    if (typeof window === 'undefined') return
    try {
      const saved = localStorage.getItem('attackbox-appearance') as ThemeMode | null
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
