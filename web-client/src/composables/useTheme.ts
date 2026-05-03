import { ref, watch } from 'vue'

const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('theme') : null
export const isDark = ref(stored !== 'light')

watch(isDark, (v) => {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', v ? 'dark' : 'light')
    localStorage.setItem('theme', v ? 'dark' : 'light')
  }
})

export function useTheme() {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', isDark.value ? 'dark' : 'light')
  }
}

export function toggleTheme() {
  isDark.value = !isDark.value
}
