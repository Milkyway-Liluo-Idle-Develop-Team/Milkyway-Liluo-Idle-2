import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import './styles/theme.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)

app.mount('#app')

// Expose internals to E2E tests in dev mode.
if (import.meta.env.DEV) {
  setTimeout(async () => {
    const { useGameStore } = await import('./stores/game')
    const actions = await import('./lib/actions')
    ;(window as any).__gameStore = useGameStore()
    ;(window as any).__actions = actions
  }, 500)
}
