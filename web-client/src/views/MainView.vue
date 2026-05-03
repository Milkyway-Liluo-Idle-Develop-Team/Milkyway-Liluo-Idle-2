<template>
  <div class="main-page">
    <!-- Top bar -->
    <header class="top-bar">
      <div class="brand" @click="router.push('/main')">MLI</div>
      <div class="loop-bar" v-if="store.activeLoop">
        <span class="loop-name">{{ eventName(store.activeLoop.event_id) }}</span>
        <div class="progress-track">
          <div class="progress-fill" :style="{ width: `${(store.activeLoop.elapsed_seconds / store.activeLoop.duration_seconds) * 100}%` }" />
        </div>
        <span class="loop-percent">
          {{ ((store.activeLoop.elapsed_seconds / store.activeLoop.duration_seconds) * 100).toFixed(0) }}%
        </span>
      </div>
      <div class="spacer" />
      <div class="actions">
        <button @click="router.push('/battle')">战斗</button>
        <button @click="router.push('/user')">用户</button>
        <button class="theme-btn" @click="toggleTheme()">{{ isDark ? '☀️' : '🌙' }}</button>
      </div>
    </header>

    <!-- Queue bar -->
    <div class="queue-bar" v-if="store.queue.items.length > 0">
      <div class="queue-list">
        <div
          v-for="(item, idx) in store.queue.items"
          :key="idx"
          :class="['queue-item', { current: idx === 0 }]"
        >
          <span class="q-name">{{ item.name }}</span>
          <span v-if="item.iterations" class="q-iter">×{{ item.iterations }}</span>
          <button v-if="idx > 0" class="q-btn" @click="store.queueRemove(idx)">×</button>
        </div>
      </div>
    </div>

    <!-- Main layout -->
    <div class="layout">
      <!-- Left sidebar -->
      <aside class="sidebar left">
        <h3>生产技能</h3>
        <div class="skill-list">
          <button
            v-for="sk in store.productionSkills"
            :key="sk.id"
            :class="['skill-btn', { active: selectedSkill === sk.id }]"
            @click="selectedSkill = sk.id"
          >
            <div class="skill-ring" :style="ringStyle(sk.level_progress)">
              <span class="skill-lv">{{ sk.level }}</span>
            </div>
            <span class="skill-label">{{ sk.name }}</span>
          </button>
        </div>
        <h3>战斗技能</h3>
        <div class="skill-list">
          <div v-for="sk in store.combatSkills" :key="sk.id" class="skill-row">
            <span>{{ sk.name }}</span>
            <span class="muted">Lv.{{ sk.level }}</span>
          </div>
        </div>
      </aside>

      <!-- Center -->
      <main class="center">
        <div class="filters">
          <select v-model="selectedMap">
            <option value="">所有地图</option>
            <option v-for="m in maps" :key="m.id" :value="m.id">{{ m.name }}</option>
          </select>
        </div>

        <div class="section-title">循环事件</div>
        <div class="event-grid">
          <div
            v-for="ev in filteredLoopEvents"
            :key="ev.id"
            class="event-card"
            @click="selectedEvent = ev.id"
          >
            <div class="event-header">
              <strong>{{ ev.name }}</strong>
              <span class="badge loop">循环</span>
            </div>
            <div class="event-body">
              <p>{{ ev.description }}</p>
              <div v-if="ev.loop_time" class="meta">⏱ {{ ev.loop_time }}秒</div>
              <div v-if="ev.cost_items.length" class="costs">
                <span v-for="c in ev.cost_items" :key="c.item_id">{{ c.item_name }} ×{{ c.value }}</span>
              </div>
              <div v-if="ev.reward_preview.length" class="rewards">
                <span v-for="r in ev.reward_preview" :key="r.item_id">{{ r.item_name }} ×{{ r.base_value }}</span>
              </div>
            </div>
            <div class="event-actions">
              <button @click.stop="store.queueAppend(ev.id)">加入队列</button>
              <button @click.stop="store.queueAppend(ev.id, 1)">执行1次</button>
            </div>
          </div>
        </div>

        <div class="section-title">升级事件</div>
        <div class="event-grid">
          <div
            v-for="ev in filteredUpgradeEvents"
            :key="ev.id"
            class="event-card"
          >
            <div class="event-header">
              <strong>{{ ev.name }}</strong>
              <span class="badge upgrade">升级</span>
            </div>
            <div class="event-body">
              <p>{{ ev.description }}</p>
              <div v-if="ev.cost_items.length" class="costs">
                <span v-for="c in ev.cost_items" :key="c.item_id">{{ c.item_name }} ×{{ c.value }}</span>
              </div>
            </div>
            <div class="event-actions">
              <button @click.stop="store.queueAppend(ev.id, 1)">执行</button>
            </div>
          </div>
        </div>
      </main>

      <!-- Right sidebar -->
      <aside class="sidebar right">
        <h3>背包</h3>
        <div v-for="panel in store.itemPanels" :key="panel.classification" class="item-panel">
          <div class="panel-title">{{ panel.title }}</div>
          <div class="item-grid">
            <div v-for="it in panel.items" :key="it.id" class="item-cell">
              <img :src="`/icons/items/${it.id}.svg`" class="item-icon" @error="onImgError" />
              <div class="item-qty">{{ formatQty(it.quantity) }}</div>
            </div>
          </div>
        </div>

        <h3>装备</h3>
        <div class="equip-section">
          <div class="equip-group">
            <div v-for="slot in store.equipmentView.production_slots" :key="slot.slot_id" class="equip-slot">
              <div class="slot-name">{{ slot.slot_name }}</div>
              <div v-if="slot.item_name" class="slot-item">{{ slot.item_name }}</div>
              <div v-else class="slot-empty">空</div>
            </div>
          </div>
          <div class="equip-group">
            <div v-for="slot in store.equipmentView.battle_slots" :key="slot.slot_id" class="equip-slot">
              <div class="slot-name">{{ slot.slot_name }}</div>
              <div v-if="slot.item_name" class="slot-item">{{ slot.item_name }}</div>
              <div v-else class="slot-empty">空</div>
            </div>
          </div>
        </div>
      </aside>
    </div>

    <div v-if="store.actionError" class="toast-error" @click="store.clearActionError()">
      {{ store.actionError }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useGameStore } from '@/stores/game'
import { toggleTheme, isDark } from '@/composables/useTheme'
import { connect, disconnect } from '@/lib/ws'

const router = useRouter()
const store = useGameStore()

const selectedSkill = ref<string>('felling')
const selectedMap = ref<string>('')
const selectedEvent = ref<string | null>(null)

const maps = computed(() => {
  const seen = new Set<string>()
  const out: Array<{ id: string; name: string }> = []
  for (const ev of store.config?.actions.events ?? []) {
    if (seen.has(ev.map)) continue
    seen.add(ev.map)
    out.push({ id: ev.map, name: MAP_NAME_MAP[ev.map] || ev.map })
  }
  return out
})

const filteredLoopEvents = computed(() => {
  return store.loopEvents.filter((ev) => {
    if (selectedSkill.value && ev.need_skill !== selectedSkill.value) return false
    if (selectedMap.value && ev.map !== selectedMap.value) return false
    return true
  })
})

const filteredUpgradeEvents = computed(() => {
  return store.upgradeEvents.filter((ev) => {
    if (selectedMap.value && ev.map !== selectedMap.value) return false
    return true
  })
})

function eventName(id: string) {
  return store.config?.actions.events.find((e) => e.id === id)?.name || id
}

function ringStyle(progress: number) {
  const deg = Math.round(progress * 360)
  return {
    background: `conic-gradient(var(--brand) ${deg}deg, var(--surface-3) ${deg}deg)`,
  }
}

function formatQty(n: number) {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(Math.floor(n))
}

function onImgError(e: Event) {
  const target = e.target as HTMLImageElement
  target.style.display = 'none'
}

onMounted(async () => {
  await store.fetchStaticData()
  store.initWsListeners()
  try {
    await connect()
  } catch {
    // ignore
  }
})

onUnmounted(() => {
  store.disposeWsListeners()
})
</script>

<style scoped>
.main-page {
  min-height: 100vh;
  background: var(--bg);
  color: var(--text);
  display: flex;
  flex-direction: column;
}

.top-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 10px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
}
.brand {
  font-size: 20px;
  font-weight: 800;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  cursor: pointer;
}
.loop-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  max-width: 400px;
}
.loop-name {
  font-size: 13px;
  white-space: nowrap;
}
.progress-track {
  flex: 1;
  height: 8px;
  background: var(--bg-2);
  border-radius: 4px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--brand), var(--brand-2));
  border-radius: 4px;
  transition: width 0.3s ease;
}
.loop-percent {
  font-size: 12px;
  color: var(--muted);
  min-width: 36px;
  text-align: right;
}
.spacer { flex: 1; }
.actions {
  display: flex;
  gap: 8px;
}
.actions button {
  padding: 6px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  color: var(--text);
  cursor: pointer;
  font-size: 13px;
}
.theme-btn {
  font-size: 16px !important;
}

.queue-bar {
  padding: 8px 20px;
  border-bottom: 1px solid var(--border);
  background: var(--surface);
  overflow-x: auto;
}
.queue-list {
  display: flex;
  gap: 8px;
}
.queue-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-radius: 8px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  font-size: 12px;
  white-space: nowrap;
}
.queue-item.current {
  border-color: var(--brand);
  background: color-mix(in srgb, var(--brand) 12%, var(--surface-2));
}
.q-btn {
  background: none;
  border: none;
  color: var(--danger);
  cursor: pointer;
  font-size: 14px;
  padding: 0 2px;
}
.q-iter { color: var(--muted); }

.layout {
  display: grid;
  grid-template-columns: 220px 1fr 260px;
  gap: 16px;
  padding: 16px;
  flex: 1;
}
@media (max-width: 1100px) {
  .layout { grid-template-columns: 1fr; }
  .sidebar { display: none; }
}

.sidebar {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 14px;
  height: fit-content;
}
.sidebar h3 {
  margin: 0 0 10px;
  font-size: 14px;
  color: var(--muted);
}
.skill-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 16px;
}
.skill-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  color: var(--text);
  cursor: pointer;
}
.skill-btn.active {
  border-color: var(--brand);
}
.skill-ring {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 700;
}
.skill-label { font-size: 13px; }
.skill-row {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  padding: 4px 0;
}
.muted { color: var(--muted); }

.center {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 16px;
}
.filters {
  margin-bottom: 12px;
}
.filters select {
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text);
  font-size: 13px;
}
.section-title {
  font-size: 14px;
  font-weight: 600;
  margin: 16px 0 10px;
  color: var(--muted);
}
.event-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
}
.event-card {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 12px;
  cursor: pointer;
  transition: border-color 0.15s;
}
.event-card:hover {
  border-color: var(--brand);
}
.event-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}
.badge {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 999px;
  font-weight: 600;
}
.badge.loop { background: var(--brand); color: #fff; }
.badge.upgrade { background: var(--purple); color: #fff; }
.event-body p {
  margin: 0 0 8px;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.4;
}
.meta {
  font-size: 12px;
  color: var(--muted-2);
  margin-bottom: 6px;
}
.costs, .rewards {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 6px;
}
.costs span, .rewards span {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 6px;
  background: var(--bg);
  color: var(--muted);
}
.event-actions {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}
.event-actions button {
  flex: 1;
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
  font-size: 12px;
}
.event-actions button:hover {
  background: var(--brand);
  color: #fff;
  border-color: var(--brand);
}

.item-panel {
  margin-bottom: 12px;
}
.panel-title {
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 6px;
}
.item-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 6px;
}
.item-cell {
  aspect-ratio: 1;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
  padding: 4px;
}
.item-icon {
  width: 32px;
  height: 32px;
}
.item-qty {
  position: absolute;
  bottom: 2px;
  right: 4px;
  font-size: 10px;
  color: var(--muted);
}

.equip-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.equip-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.equip-slot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 8px;
  border-radius: 6px;
  background: var(--surface-2);
  font-size: 12px;
}
.slot-name { color: var(--muted); }
.slot-item { color: var(--brand); }
.slot-empty { color: var(--muted-2); }

.toast-error {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: var(--danger);
  color: #fff;
  padding: 10px 20px;
  border-radius: 10px;
  font-size: 13px;
  cursor: pointer;
  z-index: 100;
}
</style>
