import { createRouter, createWebHistory } from 'vue-router'
import { isAuthenticated } from '@/lib/auth'
import AuthView from '@/views/AuthView.vue'
import MainView from '@/views/MainView.vue'
import BattleView from '@/views/BattleView.vue'
import UserView from '@/views/UserView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/login' },
    { path: '/login', name: 'login', component: AuthView, props: { mode: 'login' } },
    { path: '/register', name: 'register', component: AuthView, props: { mode: 'register' } },
    { path: '/main', name: 'main', component: MainView, meta: { requiresAuth: true } },
    { path: '/battle', name: 'battle', component: BattleView, meta: { requiresAuth: true } },
    { path: '/actions', redirect: '/main' },
    { path: '/user', name: 'user', component: UserView, meta: { requiresAuth: true } },
    { path: '/:pathMatch(.*)*', redirect: '/login' },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta?.requiresAuth) {
    const ok = await isAuthenticated()
    if (!ok) {
      return { path: '/login', query: { redirect: to.fullPath } }
    }
    // WS 连接延迟到 MainView 加载完静态配置后再建立，
    // 避免 state.full 在 idRegistry 就绪前到达而被丢弃。
  }

  if ((to.path === '/login' || to.path === '/register') && (await isAuthenticated())) {
    return { path: '/main' }
  }

  return true
})

export default router
