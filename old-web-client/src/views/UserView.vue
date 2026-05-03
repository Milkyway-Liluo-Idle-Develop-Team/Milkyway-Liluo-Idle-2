<template>
  <div class="page">
    <header class="header">
      <div class="title">
        <h1>用户中心</h1>
        <p class="subtitle" v-if="me?.username">{{ me.username }}（UID: {{ me.uid }}）</p>
      </div>

      <div class="actions">
        <RouterLink class="btn" to="/main">返回面板</RouterLink>
        <button class="btn danger" type="button" @click="onLogout" :disabled="loggingOut">
          {{ loggingOut ? '登出中...' : '登出' }}
        </button>
      </div>
    </header>

    <nav class="tabs" role="tablist" aria-label="用户信息菜单">
      <button
        class="tab"
        type="button"
        role="tab"
        :aria-selected="active === 'account'"
        :class="{ active: active === 'account' }"
        @click="active = 'account'"
      >
        账号
      </button>
      <button
        class="tab"
        type="button"
        role="tab"
        :aria-selected="active === 'inventory'"
        :class="{ active: active === 'inventory' }"
        @click="active = 'inventory'"
      >
        背包
      </button>
      <button
        class="tab"
        type="button"
        role="tab"
        :aria-selected="active === 'skills'"
        :class="{ active: active === 'skills' }"
        @click="active = 'skills'"
      >
        技能
      </button>
    </nav>

    <section class="card" v-if="error">
      <p class="error">{{ error }}</p>
    </section>

    <section class="card" v-if="active === 'account'">
      <div class="card-title">账号基本信息</div>
      <div v-if="loadingMe" class="muted">加载中...</div>
      <div v-else-if="me" class="grid">
        <div class="row">
          <div class="k">UID</div>
          <div class="v">{{ me.uid }}</div>
        </div>
        <div class="row">
          <div class="k">用户名</div>
          <div class="v">{{ me.username }}</div>
        </div>
        <div class="row">
          <div class="k">邮箱</div>
          <div class="v">{{ me.email }}</div>
        </div>
        <div class="row">
          <div class="k">创建时间</div>
          <div class="v">{{ createdAtText }}</div>
        </div>
      </div>
    </section>

    <section class="card" v-if="active === 'inventory'">
      <div class="card-title">背包信息</div>
      <div v-if="store.state && store.state.inventory.length" class="list">
        <div v-for="item in store.state.inventory" :key="`${item.id}:${item.state ?? 0}`" class="item">
          <span class="name">{{ item.id }}</span>
          <span class="qty">× {{ item.qty }}</span>
        </div>
      </div>
      <div v-else class="muted">背包为空</div>
    </section>

    <section class="card" v-if="active === 'skills'">
      <div class="card-title">技能信息</div>
      <div v-if="store.state && Object.keys(store.state.skills).length" class="list">
        <div v-for="(sk, id) in store.state.skills" :key="id" class="item">
          <span class="name">{{ id }}</span>
          <span class="qty">Lv {{ sk.level }} · EXP {{ formatExp(sk.exp) }}</span>
        </div>
      </div>
      <div v-else class="muted">暂无技能数据</div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { getJson } from '@/lib/api'
import { clearAuthCache, logout } from '@/lib/auth'
import { useGameStore } from '@/stores/game'

type Me = { uid: number; username: string; email: string; created_at: number }

const router = useRouter()

const active = ref<'account' | 'inventory' | 'skills'>('account')
const error = ref('')

const me = ref<Me | null>(null)
const loadingMe = ref(false)
const loggingOut = ref(false)

const createdAtText = computed(() => {
  if (!me.value?.created_at) return '--'
  const ms = Number(me.value.created_at) * 1000
  const d = new Date(ms)
  if (Number.isNaN(d.getTime())) return String(me.value.created_at)
  return d.toLocaleString()
})

const formatExp = (value: unknown) => {
  const num = Number(value)
  if (!Number.isFinite(num)) return '--'
  return num.toFixed(1)
}

const loadMe = async () => {
  loadingMe.value = true
  try {
    const res = await getJson<{ success: true; user: Me }>('/api/me', { credentials: 'include' })
    if (!res.ok) {
      error.value = res.error
      return
    }
    me.value = res.data.user
  } finally {
    loadingMe.value = false
  }
}

const store = useGameStore()

const onLogout = async () => {
  loggingOut.value = true
  try {
    const res = await logout()
    clearAuthCache()
    store.disposeWsListeners()
    store.resetState()
    if (!res.ok) {
      error.value = res.error
      return
    }
    await router.replace('/login')
  } finally {
    loggingOut.value = false
  }
}

onMounted(() => {
  loadMe()
})
</script>

<style scoped>
.page {
  max-width: 720px;
  margin: 0 auto;
  padding: 24px 16px 80px;
}

.header {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
}

.title h1 {
  margin: 0;
  font-size: 1.5rem;
}

.subtitle {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 0.875rem;
}

.actions {
  display: flex;
  gap: 8px;
}

.btn {
  padding: 8px 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--bg-2);
  color: var(--text);
  font-size: 0.875rem;
  text-decoration: none;
  cursor: pointer;
}

.btn.danger {
  border-color: transparent;
  background: hsl(0deg 70% 46%);
  color: #fff;
}

.btn.danger:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.tabs {
  display: flex;
  gap: 0;
  border-bottom: 2px solid var(--border);
  margin-bottom: 20px;
}

.tab {
  padding: 8px 20px;
  border: none;
  background: transparent;
  color: var(--muted);
  font-size: 0.95rem;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  margin-bottom: -2px;
  transition: color 0.18s, border-color 0.18s;
}

.tab:hover {
  color: var(--text);
}

.tab.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
}

.card {
  background: var(--bg-2);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
}

.card-title {
  font-weight: 600;
  font-size: 1.05rem;
  margin-bottom: 12px;
}

.grid {
  display: grid;
  gap: 8px;
}

.row {
  display: flex;
  gap: 12px;
  padding: 4px 0;
}

.k {
  width: 100px;
  color: var(--muted);
  flex-shrink: 0;
}

.v {
  word-break: break-all;
}

.list {
  display: grid;
  gap: 6px;
}

.item {
  display: flex;
  gap: 12px;
  justify-content: space-between;
  padding: 6px 0;
  border-bottom: 1px solid var(--border);
}

.item:last-child {
  border-bottom: none;
}

.name {
  font-weight: 500;
}

.qty {
  color: var(--muted);
  font-size: 0.9rem;
}

.error {
  color: hsl(0deg 85% 58%);
  font-weight: 500;
}

.muted {
  color: var(--muted);
}
</style>
