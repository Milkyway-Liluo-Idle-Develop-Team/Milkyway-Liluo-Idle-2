import { computed, ref } from 'vue'

type ThemeName = 'dark' | 'light'

const getInitialTheme = (): ThemeName => {
  if (typeof window === 'undefined') return 'dark'
  const saved = window.localStorage.getItem('theme')
  if (saved === 'dark' || saved === 'light') return saved
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const theme = ref<ThemeName>(getInitialTheme())

const applyTheme = (value: ThemeName) => {
  if (typeof document === 'undefined') return
  document.documentElement.dataset.theme = value
  window.localStorage.setItem('theme', value)
}

applyTheme(theme.value)

export function useTheme() {
  const toggleTheme = () => {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    applyTheme(theme.value)
  }

  const themeAriaLabel = computed(() =>
    theme.value === 'dark' ? '切换到亮色主题' : '切换到暗色主题',
  )

  return { theme, toggleTheme, themeAriaLabel }
}

