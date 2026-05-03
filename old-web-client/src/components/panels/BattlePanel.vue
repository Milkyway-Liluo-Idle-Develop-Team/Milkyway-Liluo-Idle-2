<template>
  <div class="scene-col">
    <h2>战斗地图</h2>
    <button
      v-for="scene in mapTabs"
      :key="scene.id"
      type="button"
      class="scene-btn"
      :class="{ active: selectedMapId === scene.id }"
      @click="$emit('selectMap', scene.id)"
    >
      {{ scene.name }}
    </button>
  </div>

  <div class="loop-col">
    <h2>战斗</h2>
    <p v-if="visibleError" class="error">{{ visibleError }}</p>
    <div v-else class="events-list">
      <article
        v-for="entry in visibleBattles"
        :key="entry.id"
        class="event-card"
        :class="{ blocked: battleRunning && activeBattleId !== entry.id }"
      >
        <div class="event-head">
          <strong>{{ entry.name }}</strong>
          <span class="tag" :class="{ blocked: battleRunning && activeBattleId !== entry.id }">
            {{ activeBattleId === entry.id ? '战斗中' : '待命' }}
          </span>
        </div>

        <p>地图 {{ mapName(entry.map) }} · 间隔 {{ formatSeconds(entry.interval) }}</p>

        <div class="event-actions">
          <button
            class="event-action"
            type="button"
            :disabled="battleActionLoading || (battleRunning && activeBattleId !== entry.id)"
            @click="$emit('toggleBattle', entry)"
          >
            {{ activeBattleId === entry.id ? '停止战斗' : '开始战斗' }}
          </button>
          <button class="event-action ghost" type="button" :disabled="battleActionLoading" @click="$emit('goBattlePage')">
            进入战斗页
          </button>
        </div>
      </article>

      <div v-if="!visibleBattles.length" class="empty">当前地图暂无可用战斗</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { BattleListItem } from '@/types/BattleResponse'

const props = defineProps<{
  entries: BattleListItem[]
  selectedMapId: string
  battleRunning: boolean
  activeBattleId: string
  battleActionLoading: boolean
  visibleError: string
  maps: Array<{ id: string; name: string }>
}>()

defineEmits<{
  selectMap: [mapId: string]
  toggleBattle: [entry: BattleListItem]
  goBattlePage: []
}>()

const mapTabs = computed(() => {
  const mapNames = new Map(props.maps.map((entry) => [entry.id, entry.name]))
  const seen = new Set<string>()
  const out: Array<{ id: string; name: string }> = []
  for (const entry of props.entries) {
    if (seen.has(entry.map)) continue
    seen.add(entry.map)
    out.push({ id: entry.map, name: mapNames.get(entry.map) ?? entry.map })
  }
  return out
})

const visibleBattles = computed(() =>
  props.entries.filter((entry) => entry.map === props.selectedMapId),
)

const mapName = (mapId: string) => props.maps.find((entry) => entry.id === mapId)?.name ?? mapId

const formatSeconds = (value: number) => {
  const safe = Number.isFinite(value) ? Math.max(0, value) : 0
  if (safe >= 10) return `${safe.toFixed(1)}s`
  if (safe >= 1) return `${safe.toFixed(2)}s`
  return `${safe.toFixed(3)}s`
}
</script>

<style scoped>
.scene-col,
.loop-col {
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.scene-btn {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
  color: var(--text);
  font-weight: 700;
  min-height: 40px;
  cursor: pointer;
}

.scene-btn.active {
  border-color: color-mix(in srgb, var(--brand) 44%, var(--border));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--brand-2) 30%, transparent);
}

.events-list {
  overflow: auto;
  min-height: 0;
  display: grid;
  gap: 8px;
}

.event-card {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 10px;
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
}

.event-card.blocked {
  border-color: color-mix(in srgb, var(--warning) 30%, var(--border));
}

.event-head {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  align-items: center;
}

.event-card p {
  margin: 8px 0 0;
  color: var(--muted);
  line-height: 1.45;
  font-size: 0.9rem;
}

.tag {
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 0.72rem;
  border: 1px solid color-mix(in srgb, var(--success) 45%, var(--border));
  color: var(--success);
}

.tag.blocked {
  border-color: color-mix(in srgb, var(--warning) 45%, var(--border));
  color: var(--warning);
}

.event-actions {
  margin-top: 10px;
  display: flex;
  gap: 8px;
}

.event-action {
  margin-top: 10px;
  min-height: 34px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  padding: 0 12px;
  color: rgba(255, 255, 255, 0.95);
  font-weight: 800;
  cursor: pointer;
  background: linear-gradient(
    135deg,
    color-mix(in srgb, var(--brand) 82%, #000),
    color-mix(in srgb, var(--brand-2) 78%, #000)
  );
}

.event-action.ghost {
  background: color-mix(in srgb, var(--surface-2) 86%, transparent);
  color: var(--text);
  border-color: var(--border);
}

.event-action:disabled {
  background: color-mix(in srgb, var(--surface-3) 86%, transparent);
  color: var(--muted);
  border-color: var(--border);
  cursor: not-allowed;
}

.error {
  color: var(--danger);
  font-weight: 800;
  margin: 0;
}

.empty {
  color: var(--muted);
  border: 1px dashed var(--border);
  border-radius: 10px;
  padding: 10px;
}
</style>
