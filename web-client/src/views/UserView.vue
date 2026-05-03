<template>
  <div class="user-page">
    <header class="top-bar">
      <h1>用户中心</h1>
      <div class="actions">
        <button @click="router.push('/main')">返回游戏</button>
        <button @click="onLogout">退出登录</button>
      </div>
    </header>
    <div class="tabs">
      <button :class="{ active: tab === 'account' }" @click="tab = 'account'">账号</button>
      <button :class="{ active: tab === 'inventory' }" @click="tab = 'inventory'">背包</button>
      <button :class="{ active: tab === 'skills' }" @click="tab = 'skills'">技能</button>
    </div>
    <div class="content">
      <div v-if="tab === 'account'" class="panel">
        <div class="info-row"><label>UID</label><span>{{ userProfile.uid }}</span></div>
        <div class="info-row"><label>用户名</label><span>{{ userProfile.username }}</span></div>
        <div class="info-row"><label>邮箱</label><span>{{ userProfile.email }}</span></div>
      </div>
      <div v-if="tab === 'inventory'" class="panel">
        <div v-for="panel in store.itemPanels" :key="panel.classification" class="item-panel">
          <h4>{{ panel.title }}</h4>
          <div class="item-grid">
            <div v-for="it in panel.items" :key="it.id" class="item-cell">
              <img :src="`/icons/items/${it.id}.svg`" class="item-icon" />
              <div class="item-name">{{ it.name }}</div>
              <div class="item-qty">{{ formatQty(it.quantity) }}</div>
            </div>
          </div>
        </div>
      </div>
      <div v-if="tab === 'skills'" class="panel">
        <h3>生产技能</h3>
        <div class="skill-grid">
          <div v-for="sk in store.productionSkills" :key="sk.id" class="skill-cell">
            <div class="skill-name">{{ sk.name }}</div>
            <div class="skill-level">Lv.{{ sk.level }}</div>
            <div class="skill-bar"><div class="skill-fill" :style="{ width: `${sk.level_progress * 100}%` }" /></div>
          </div>
        </div>
        <h3>战斗技能</h3>
        <div class="skill-grid">
          <div v-for="sk in store.combatSkills" :key="sk.id" class="skill-cell">
            <div class="skill-name">{{ sk.name }}</div>
            <div class="skill-level">Lv.{{ sk.level }}</div>
            <div class="skill-bar"><div class="skill-fill" :style="{ width: `${sk.level_progress * 100}%` }" /></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { userProfile, logout } from '@/lib/auth'
import { useGameStore } from '@/stores/game'
import { disconnect } from '@/lib/ws'

const router = useRouter()
const store = useGameStore()
const tab = ref<'account' | 'inventory' | 'skills'>('account')

function formatQty(n: number) {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(Math.floor(n))
}

async function onLogout() {
  await logout()
  disconnect()
  store.disposeWsListeners()
  store.resetState()
  router.push('/login')
}
</script>

<style scoped>
.user-page {
  min-height: 100vh;
  background: var(--bg);
  color: var(--text);
}
.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border);
}
.top-bar h1 { margin: 0; font-size: 20px; }
.actions { display: flex; gap: 8px; }
.actions button {
  padding: 8px 14px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
}
.tabs {
  display: flex;
  gap: 8px;
  padding: 12px 24px;
  border-bottom: 1px solid var(--border);
}
.tabs button {
  padding: 8px 16px;
  border-radius: 8px;
  border: none;
  background: var(--surface-2);
  color: var(--muted);
  cursor: pointer;
}
.tabs button.active {
  background: var(--brand);
  color: #fff;
}
.content { padding: 20px 24px; }
.panel { max-width: 800px; }
.info-row {
  display: flex;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid var(--border);
}
.item-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(90px, 1fr));
  gap: 10px;
}
.item-cell {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 10px;
  text-align: center;
}
.item-icon { width: 40px; height: 40px; }
.item-name { font-size: 12px; margin-top: 4px; }
.item-qty { font-size: 11px; color: var(--muted); }
.skill-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
  margin-bottom: 20px;
}
.skill-cell {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 12px;
}
.skill-bar {
  height: 6px;
  background: var(--bg-2);
  border-radius: 3px;
  margin-top: 8px;
  overflow: hidden;
}
.skill-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  border-radius: 3px;
}
</style>
