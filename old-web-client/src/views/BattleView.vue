<template>
  <div class="battle-page">
    <header class="top panel">
      <div class="title">
        <h1>战斗页面</h1>
        <p>实时战斗（每次行动向后端同步）</p>
      </div>
      <div class="head-actions">
        <RouterLink class="btn ghost" to="/main">返回主界面</RouterLink>
      </div>
    </header>

    <section class="control panel">
      <div class="control-row">
        <label for="battleSelect">战斗场景</label>
        <select id="battleSelect" v-model="selectedBattleId" :disabled="running">
          <option v-for="item in battles" :key="item.id" :value="item.id">
            {{ item.name }} ({{ item.map }})
          </option>
        </select>
      </div>
      <div class="control-buttons">
        <button class="btn primary" type="button" :disabled="!selectedBattleId || running || loading" @click="startBattle">
          开始战斗
        </button>
        <button class="btn danger" type="button" :disabled="!running || loading" @click="stopBattle">
          停止战斗
        </button>
      </div>
      <p class="status-line" v-if="battleState">
        波次 {{ battleState.wave_number }} · 状态 {{ statusText }} · 下一次行动 {{ nextActionText }}
      </p>
      <p v-if="error" class="error">{{ error }}</p>
    </section>

    <section class="arena panel" v-if="battleState">
      <article class="team ally">
        <h2>友方</h2>
        <div ref="playerCardRef" class="unit-card" :class="{ dead: !battleState.player.alive }">
          <div class="portrait-slot">🧙</div>
          <div class="unit-info">
            <h3>{{ battleState.player.name }}</h3>
            <p class="unit-state">{{ battleState.player.alive ? '战斗中' : '已倒下' }}</p>
            <div class="bar-row">
              <span>HP</span>
              <div class="bar track-hp"><div class="fill hp" :style="barStyle(displayPlayer.hp, displayPlayer.max_hp)"></div></div>
              <strong>{{ fixed(battleState.player.hp) }} / {{ fixed(battleState.player.max_hp) }}</strong>
            </div>
            <div class="bar-row">
              <span>MP</span>
              <div class="bar track-mp"><div class="fill mp" :style="barStyle(displayPlayer.mp, displayPlayer.max_mp)"></div></div>
              <strong>{{ fixed(battleState.player.mp) }} / {{ fixed(battleState.player.max_mp) }}</strong>
            </div>
            <div class="bar-row">
              <span>SP</span>
              <div class="bar track-sp"><div class="fill sp" :style="barStyle(displayPlayer.sp, displayPlayer.max_sp)"></div></div>
              <strong>{{ fixed(battleState.player.sp) }} / {{ fixed(battleState.player.max_sp) }}</strong>
            </div>
            <div class="bar-row cooldown-row">
              <span>CD</span>
              <div class="bar track-cd">
                <div class="fill cd" :style="cooldownStyle(battleState.player.action_cooldown_progress)"></div>
              </div>
              <strong>{{ battleState.player.last_skill_name || '基础攻击' }} · {{ fixed(battleState.player.next_ready_in_seconds) }}s</strong>
            </div>
          </div>
        </div>
      </article>

      <article class="team enemy">
        <h2>敌方</h2>
        <div class="enemy-grid">
          <div
            v-for="enemy in battleState.enemies"
            :key="enemy.instance_id"
            :ref="(el) => setEnemyCardRef(enemy.instance_id, el as Element | null)"
            class="enemy-card"
            :class="{ dead: !enemy.alive }"
          >
            <div class="portrait-slot enemy-slot">👾</div>
            <strong>{{ enemy.name }}</strong>
            <div class="bar track-hp"><div class="fill hp" :style="barStyle(enemyDisplayHp(enemy), enemy.max_hp)"></div></div>
            <span class="enemy-hp">{{ fixed(enemy.hp) }} / {{ fixed(enemy.max_hp) }}</span>
            <div class="bar track-cd enemy-cd"><div class="fill cd" :style="cooldownStyle(enemy.action_cooldown_progress)"></div></div>
            <span class="enemy-skill">{{ enemy.last_skill_name || '基础攻击' }} · {{ fixed(enemy.next_ready_in_seconds) }}s</span>
          </div>
          <div v-if="!battleState.enemies.length" class="empty">当前无敌人（等待下一波）</div>
        </div>
      </article>
    </section>

    <div class="fx-layer">
      <svg class="arc-layer" :viewBox="`0 0 ${viewportSize.w} ${viewportSize.h}`" preserveAspectRatio="none">
        <path
          v-for="arc in attackArcs"
          :key="arc.id"
          :d="arc.path"
          :style="{ stroke: arc.color }"
          class="attack-arc"
        />
      </svg>
      <div
        v-for="popup in damagePopups"
        :key="popup.id"
        class="damage-popup"
        :class="popup.kind"
        :style="{ left: `${popup.x}px`, top: `${popup.y}px` }"
      >
        {{ popup.text }}
      </div>
    </div>

    <section class="panel log-panel">
      <h2>战斗日志</h2>
      <div class="logs">
        <p v-for="(line, idx) in renderedLogs" :key="`${idx}-${line}`">{{ line }}</p>
        <p v-if="!renderedLogs.length" class="muted">暂无日志</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { onStatusChange } from '@/lib/ws'
import * as battleActions from '@/lib/actions'
import type { BattleListItem, BattleState } from '@/types/BattleResponse'

const battles = ref<BattleListItem[]>([])
const selectedBattleId = ref('')
const battleState = ref<BattleState | null>(null)
const loading = ref(false)
const running = ref(false)
const error = ref('')
const logLines = ref<string[]>([])
const playerCardRef = ref<HTMLElement | null>(null)
const enemyCardRefMap = new Map<string, HTMLElement>()

const displayPlayer = ref({ hp: 0, max_hp: 1, mp: 0, max_mp: 1, sp: 0, max_sp: 1 })
const displayEnemyMap = ref<Record<string, number>>({})

type DamagePopup = {
  id: string
  x: number
  y: number
  text: string
  kind: 'enemy-hit' | 'player-hit'
}

type AttackArc = {
  id: string
  path: string
  color: string
}

const damagePopups = ref<DamagePopup[]>([])
const attackArcs = ref<AttackArc[]>([])
const viewportSize = ref({ w: window.innerWidth, h: window.innerHeight })

const BATTLE_SYNC_INTERVAL_MS = 500

let syncTimer: number | null = null
let displayTimer: number | null = null
let visualLogTimer: number | null = null
let fxSeq = 0

const pendingVisualLogs = ref<Record<string, unknown>[]>([])

const statusText = computed(() => {
  const state = battleState.value
  if (!state) return '-'
  if (state.status === 'fighting') return '战斗中'
  if (state.status === 'between_waves') return '波次间隔'
  if (state.status === 'respawn') return '等待复活'
  if (state.status === 'stopped') return '已停止'
  return state.status
})

const nextActionText = computed(() => {
  const state = battleState.value
  if (!state || state.next_step_in_seconds === null) return '-'
  return `${state.next_step_in_seconds.toFixed(2)}s`
})

const renderedLogs = computed(() => logLines.value.slice(-120))

const skillNameMap: Record<string, string> = {
  strength: '力量',
  ranging: '远程',
  resilience: '坚韧',
  stamina: '耐力',
  intelligence: '智力',
  defense: '防御',
  magic: '魔法',
}

const clearSyncTimer = () => {
  if (syncTimer !== null) {
    window.clearInterval(syncTimer)
    syncTimer = null
  }
}

const clearDisplayTimer = () => {
  if (displayTimer !== null) {
    window.clearInterval(displayTimer)
    displayTimer = null
  }
}

const updateViewportSize = () => {
  viewportSize.value = {
    w: Math.max(1, window.innerWidth),
    h: Math.max(1, window.innerHeight),
  }
}

const fixed = (v: number) => (Number.isFinite(v) ? v.toFixed(1) : '0.0')
const fmt = (v: number) => {
  if (!Number.isFinite(v)) return '0'
  if (Math.abs(v - Math.round(v)) < 1e-9) return `${Math.round(v)}`
  if (Math.abs(v) >= 10) return v.toFixed(1)
  return v.toFixed(2)
}

const barStyle = (value: number, maxValue: number) => {
  const ratio = maxValue <= 0 ? 0 : Math.max(0, Math.min(1, value / maxValue))
  return { width: `${ratio * 100}%` }
}

const cooldownStyle = (progress: number) => {
  const ratio = Math.max(0, Math.min(1, Number(progress) || 0))
  return { width: `${ratio * 100}%` }
}

const enemyDisplayHp = (enemy: BattleState['enemies'][number]) => {
  return displayEnemyMap.value[enemy.instance_id] ?? enemy.hp
}

const setEnemyCardRef = (instanceId: string, el: Element | null) => {
  if (el instanceof HTMLElement) {
    enemyCardRefMap.set(instanceId, el)
  } else {
    enemyCardRefMap.delete(instanceId)
  }
}

const syncDisplayTargets = (state: BattleState) => {
  displayPlayer.value.max_hp = Math.max(1, state.player.max_hp)
  displayPlayer.value.max_mp = Math.max(1, state.player.max_mp)
  displayPlayer.value.max_sp = Math.max(1, state.player.max_sp)

  if (displayPlayer.value.hp <= 0 && state.player.max_hp > 0) {
    displayPlayer.value.hp = state.player.hp
  }
  if (displayPlayer.value.mp <= 0 && state.player.max_mp > 0) {
    displayPlayer.value.mp = state.player.mp
  }
  if (displayPlayer.value.sp <= 0 && state.player.max_sp > 0) {
    displayPlayer.value.sp = state.player.sp
  }

  const nextEnemyDisplay: Record<string, number> = {}
  for (const enemy of state.enemies) {
    const old = displayEnemyMap.value[enemy.instance_id]
    nextEnemyDisplay[enemy.instance_id] = typeof old === 'number' && Number.isFinite(old) ? old : enemy.hp
  }
  displayEnemyMap.value = nextEnemyDisplay
}

const startDisplayTween = () => {
  if (displayTimer !== null) return
  displayTimer = window.setInterval(() => {
    const state = battleState.value
    if (!state) return

    displayPlayer.value.hp += (state.player.hp - displayPlayer.value.hp) * 0.2
    displayPlayer.value.mp += (state.player.mp - displayPlayer.value.mp) * 0.2
    displayPlayer.value.sp += (state.player.sp - displayPlayer.value.sp) * 0.2

    if (Math.abs(state.player.hp - displayPlayer.value.hp) < 0.03) displayPlayer.value.hp = state.player.hp
    if (Math.abs(state.player.mp - displayPlayer.value.mp) < 0.03) displayPlayer.value.mp = state.player.mp
    if (Math.abs(state.player.sp - displayPlayer.value.sp) < 0.03) displayPlayer.value.sp = state.player.sp

    const nextEnemyDisplay = { ...displayEnemyMap.value }
    for (const enemy of state.enemies) {
      const old = nextEnemyDisplay[enemy.instance_id] ?? enemy.hp
      let next = old + (enemy.hp - old) * 0.2
      if (Math.abs(enemy.hp - next) < 0.03) {
        next = enemy.hp
      }
      nextEnemyDisplay[enemy.instance_id] = next
    }
    displayEnemyMap.value = nextEnemyDisplay
  }, 16)
}

const logToText = (entry: Record<string, unknown>) => {
  const type = String(entry.type ?? '')
  if (type === 'battle_started') return '战斗开始'
  if (type === 'battle_stopped') return '战斗停止'
  if (type === 'wave_spawn') return `第 ${entry.wave_number ?? '?'} 波出现: ${(entry.enemies as string[] | undefined)?.join(', ') ?? ''}`
  if (type === 'player_attack') return `玩家使用 ${entry.skill_name ?? entry.skill_id}，造成 ${entry.damage ?? 0} 伤害`
  if (type === 'enemy_attack') {
    const skill = `${entry.skill_name ?? entry.skill_id ?? '基础攻击'}`
    const effects = Array.isArray(entry.effects) ? (entry.effects as Array<Record<string, unknown>>) : []
    if (Number(entry.damage ?? 0) <= 0 && effects.length > 0) {
      const txt = effects.map((e) => {
        const mode = String(e.mode ?? 'flat')
        const value = Number(e.value ?? 0)
        const attr = String(e.attribute ?? '')
        if (mode === 'percent_multiplier') {
          return `${attr}${value >= 0 ? '+' : ''}${(value * 100).toFixed(1)}%`
        }
        return `${attr}${value >= 0 ? '+' : ''}${fmt(value)}`
      })
      return `${entry.enemy_name ?? entry.enemy_id} 使用 ${skill}，效果: ${txt.join('，')}`
    }
    return `${entry.enemy_name ?? entry.enemy_id} 使用 ${skill}，造成 ${entry.damage ?? 0} 伤害`
  }
  if (type === 'enemy_down') {
    const preview = Array.isArray(entry.reward_preview) ? (entry.reward_preview as Array<Record<string, unknown>>) : []
    const parts: string[] = []
    for (const rw of preview) {
      const rewardType = String(rw.type ?? '')
      if (rewardType === 'item') {
        parts.push(`${rw.item_name ?? rw.item_id} x${fmt(Number(rw.value ?? 0))}`)
      } else if (rewardType === 'experience') {
        const sid = String(rw.skill_id ?? '')
        const sn = String(rw.skill_name ?? '') || skillNameMap[sid] || sid
        parts.push(`${sn}经验 +${fmt(Number(rw.value ?? 0))}`)
      }
    }
    const suffix = parts.length ? `，掉落: ${parts.join('，')}` : ''
    return `${entry.enemy_name ?? entry.enemy_id} 被击败${suffix}`
  }
  if (type === 'player_down') return '玩家倒下，等待复活'
  if (type === 'player_respawn') return '玩家复活'
  if (type === 'wave_clear') {
    const rewards = (entry.rewards as Record<string, unknown> | undefined) ?? {}
    const itemChanges = Array.isArray(rewards.item_changes) ? (rewards.item_changes as Array<Record<string, unknown>>) : []
    const actionExp = Array.isArray(rewards.battle_action_exp) ? (rewards.battle_action_exp as Array<Record<string, unknown>>) : []
    const itemText = itemChanges
      .filter((x) => Number(x.delta ?? 0) > 0)
      .map((x) => `${x.item_name ?? x.item_id} +${fmt(Number(x.delta ?? 0))}`)
    const expText = actionExp
      .filter((x) => Number(x.value ?? 0) > 0)
      .map((x) => `${x.skill_name ?? skillNameMap[String(x.skill_id ?? '')] ?? x.skill_id}经验 +${fmt(Number(x.value ?? 0))}`)
    const extra: string[] = []
    if (itemText.length) extra.push(`战利品: ${itemText.join('，')}`)
    if (expText.length) extra.push(`战斗经验: ${expText.join('，')}`)
    if (!extra.length) return `第 ${entry.wave_number ?? '?'} 波完成，奖励已结算`
    return `第 ${entry.wave_number ?? '?'} 波完成，${extra.join('；')}`
  }
  return JSON.stringify(entry)
}

const getElementCenter = (el: HTMLElement) => {
  const rect = el.getBoundingClientRect()
  return {
    x: rect.left + rect.width / 2,
    y: rect.top + rect.height / 2,
  }
}

const spawnDamagePopup = (el: HTMLElement | null, amount: number, kind: 'enemy-hit' | 'player-hit') => {
  if (!el) return
  const center = getElementCenter(el)
  const id = `pop-${fxSeq++}`
  const text = amount <= 0 ? 'MISS' : `-${Math.max(0, Math.round(amount))}`
  damagePopups.value = [
    ...damagePopups.value,
    {
      id,
      x: center.x,
      y: center.y - 18,
      text,
      kind,
    },
  ]
  window.setTimeout(() => {
    damagePopups.value = damagePopups.value.filter((item) => item.id !== id)
  }, 920)
}

const spawnAttackArc = (fromEl: HTMLElement | null, toEl: HTMLElement | null, fromPlayer: boolean) => {
  if (!fromEl || !toEl) return
  const a = getElementCenter(fromEl)
  const b = getElementCenter(toEl)
  const dx = b.x - a.x
  const dist = Math.max(1, Math.abs(dx))
  const bulge = Math.max(46, dist * 0.22)
  const cx = (a.x + b.x) / 2
  const cy = fromPlayer ? Math.min(a.y, b.y) - bulge : Math.max(a.y, b.y) + bulge

  const id = `arc-${fxSeq++}`
  const path = `M ${a.x.toFixed(2)} ${a.y.toFixed(2)} Q ${cx.toFixed(2)} ${cy.toFixed(2)} ${b.x.toFixed(2)} ${b.y.toFixed(2)}`
  attackArcs.value = [
    ...attackArcs.value,
    {
      id,
      path,
      color: fromPlayer ? '#6dff7f' : '#ff6470',
    },
  ]
  window.setTimeout(() => {
    attackArcs.value = attackArcs.value.filter((item) => item.id !== id)
  }, 450)
}

const findEnemyElement = (enemyInstanceId: string, enemyId: string) => {
  const byInstance = enemyCardRefMap.get(enemyInstanceId)
  if (byInstance) return byInstance
  const state = battleState.value
  if (!state) return null
  const match = state.enemies.find((entry) => entry.enemy_id === enemyId)
  if (!match) return null
  return enemyCardRefMap.get(match.instance_id) ?? null
}

const processVisualLog = (entry: Record<string, unknown>) => {
  const type = String(entry.type ?? '')
  if (type === 'player_attack') {
    const targetInstanceId = String(entry.target_instance_id ?? '')
    const targetId = String(entry.target_id ?? '')
    const damage = Number(entry.damage ?? 0)
    if (damage <= 0 && Array.isArray(entry.effects) && entry.effects.length > 0) {
      return
    }
    const enemyEl = findEnemyElement(targetInstanceId, targetId)
    spawnAttackArc(playerCardRef.value, enemyEl, true)
    spawnDamagePopup(enemyEl, damage, 'enemy-hit')
    return
  }
  if (type === 'enemy_attack') {
    const enemyInstanceId = String(entry.enemy_instance_id ?? '')
    const enemyId = String(entry.enemy_id ?? '')
    const damage = Number(entry.damage ?? 0)
    if (damage <= 0 && Array.isArray(entry.effects) && entry.effects.length > 0) {
      return
    }
    const enemyEl = findEnemyElement(enemyInstanceId, enemyId)
    spawnAttackArc(enemyEl, playerCardRef.value, false)
    spawnDamagePopup(playerCardRef.value, damage, 'player-hit')
  }
}

const applyState = (state: BattleState) => {
  battleState.value = state
  syncDisplayTargets(state)
  if (state.logs?.length) {
    for (const entry of state.logs) {
      logLines.value.push(logToText(entry))
    }
    pendingVisualLogs.value.push(...state.logs)
    startVisualPlayer()
  }
}

const clearVisualLogTimer = () => {
  if (visualLogTimer !== null) {
    window.clearInterval(visualLogTimer)
    visualLogTimer = null
  }
}

const startVisualPlayer = () => {
  if (visualLogTimer !== null) return
  visualLogTimer = window.setInterval(() => {
    const batch = pendingVisualLogs.value.splice(0, 2)
    for (const entry of batch) {
      processVisualLog(entry)
    }
    if (pendingVisualLogs.value.length === 0) {
      clearVisualLogTimer()
    }
  }, 120)
}

const syncBattleState = async () => {
  if (!running.value || loading.value) return
  try {
    const state = await battleActions.syncBattleState()
    if (state) {
      applyState(state)
    } else {
      battleState.value = null
    }
    running.value = battleState.value?.status !== 'stopped'
    if (battleState.value?.status === 'stopped') {
      stopSyncTimer()
    }
  } catch {
    // ignore sync errors
  }
}

const startSyncTimer = () => {
  clearSyncTimer()
  syncTimer = window.setInterval(() => {
    void syncBattleState()
  }, BATTLE_SYNC_INTERVAL_MS)
}

const stopSyncTimer = () => {
  clearSyncTimer()
  clearVisualLogTimer()
}

const fetchBattleList = async () => {
  try {
    battles.value = await battleActions.fetchBattleList()
    if (!selectedBattleId.value) {
      selectedBattleId.value = battles.value[0]?.id ?? ''
    }
  } catch (e: any) {
    error.value = e.message || '请求失败'
  }
}

const fetchBattleState = async () => {
  try {
    const state = await battleActions.syncBattleState()
    if (state) {
      applyState(state)
    } else {
      battleState.value = null
    }
    running.value = battleState.value?.status !== 'stopped'
    if (running.value) {
      startSyncTimer()
    }
  } catch {
    // silently ignore background state fetch errors
  }
}

const startBattle = async () => {
  if (!selectedBattleId.value || loading.value) return
  error.value = ''
  loading.value = true
  try {
    const state = await battleActions.startBattle(selectedBattleId.value)
    running.value = true
    logLines.value = []
    pendingVisualLogs.value = []
    applyState(state)
    startSyncTimer()
  } catch (e: any) {
    error.value = e.message || '请求失败'
    running.value = false
  } finally {
    loading.value = false
  }
}

const stopBattle = async () => {
  if (!running.value || loading.value) return
  loading.value = true
  error.value = ''
  try {
    const state = await battleActions.stopBattle()
    applyState(state)
    running.value = false
    stopSyncTimer()
  } catch (e: any) {
    error.value = e.message || '请求失败'
  } finally {
    loading.value = false
  }
}

let unsubscribeStatus: (() => void) | null = null

onMounted(async () => {
  window.addEventListener('resize', updateViewportSize)
  updateViewportSize()
  startDisplayTween()
  await fetchBattleList()
  await fetchBattleState()

  unsubscribeStatus = onStatusChange((status) => {
    if (status === 'open') {
      fetchBattleState().catch((e) => console.warn('fetchBattleState failed:', e))
      fetchBattleList().catch((e) => console.warn('fetchBattleList failed:', e))
    }
  })
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateViewportSize)
  stopSyncTimer()
  clearDisplayTimer()
  if (unsubscribeStatus) {
    unsubscribeStatus()
    unsubscribeStatus = null
  }
})
</script>

<style scoped>
.battle-page {
  min-height: 100vh;
  padding: 20px;
  color: var(--text);
  background: linear-gradient(180deg, var(--bg), var(--bg-2));
}

.panel {
  border: 1px solid var(--border);
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface) 90%, transparent);
  box-shadow: var(--shadow-sm);
}

.top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px;
}

.title h1 {
  margin: 0;
  font-size: 1.4rem;
}

.title p {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 0.86rem;
}

.control {
  margin-top: 12px;
  padding: 12px;
  display: grid;
  gap: 10px;
}

.control-row {
  display: grid;
  gap: 6px;
}

.control-row label {
  font-weight: 700;
}

select {
  min-height: 36px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
  color: var(--text);
  padding: 0 10px;
}

.control-buttons {
  display: flex;
  gap: 10px;
}

.btn {
  min-height: 36px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-2) 80%, transparent);
  color: var(--text);
  padding: 0 14px;
  font-weight: 800;
  cursor: pointer;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
}

.btn.primary {
  color: #fff;
  background: linear-gradient(135deg, color-mix(in srgb, var(--brand) 82%, #000), color-mix(in srgb, var(--brand-2) 80%, #000));
}

.btn.danger {
  border-color: color-mix(in srgb, var(--danger) 40%, var(--border));
}

.btn:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.status-line {
  margin: 0;
  color: var(--muted);
  font-size: 0.86rem;
}

.arena {
  margin-top: 12px;
  padding: 12px;
  display: grid;
  grid-template-columns: minmax(280px, 360px) 1fr;
  gap: 12px;
}

.team {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface-2) 82%, transparent);
  padding: 10px;
  min-height: 240px;
}

.team h2 {
  margin: 0 0 8px;
  font-size: 1rem;
}

.unit-card {
  border: 1px solid var(--border);
  border-radius: 10px;
  background: color-mix(in srgb, var(--surface) 88%, transparent);
  padding: 10px;
  display: grid;
  grid-template-columns: 72px 1fr;
  gap: 10px;
}

.unit-card.dead {
  filter: saturate(0);
}

.portrait-slot {
  width: 72px;
  height: 72px;
  border: 1px dashed var(--border);
  border-radius: 10px;
  display: grid;
  place-items: center;
  font-size: 1.4rem;
}

.unit-info h3 {
  margin: 0;
  font-size: 1rem;
}

.unit-state {
  margin: 4px 0 8px;
  color: var(--muted);
  font-size: 0.8rem;
}

.bar-row {
  display: grid;
  grid-template-columns: 30px 1fr auto;
  gap: 8px;
  align-items: center;
  margin-top: 6px;
}

.bar-row span,
.bar-row strong {
  font-size: 0.76rem;
}

.bar {
  height: 10px;
  border-radius: 999px;
  border: 1px solid var(--border);
  overflow: hidden;
}

.track-hp {
  background: color-mix(in srgb, #451a1a 36%, var(--surface-2));
}

.track-mp {
  background: color-mix(in srgb, #172554 36%, var(--surface-2));
}

.track-sp {
  background: color-mix(in srgb, #3f2f16 36%, var(--surface-2));
}

.track-cd {
  background: color-mix(in srgb, #1f2937 40%, var(--surface-2));
}

.fill {
  height: 100%;
  transition: width 0.06s linear;
}

.fill.hp {
  background: linear-gradient(90deg, #ef4444, #f97316);
}

.fill.mp {
  background: linear-gradient(90deg, #2563eb, #4f46e5);
}

.fill.sp {
  background: linear-gradient(90deg, #f59e0b, #facc15);
}

.fill.cd {
  background: linear-gradient(90deg, #22c55e, #14b8a6);
}

.cooldown-row strong {
  font-size: 0.72rem;
  color: var(--muted);
}

.enemy-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.enemy-card {
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 8px;
  background: color-mix(in srgb, var(--surface) 88%, transparent);
  display: grid;
  gap: 6px;
}

.enemy-card.dead {
  filter: saturate(0);
}

.enemy-slot {
  width: 56px;
  height: 56px;
}

.enemy-hp {
  font-size: 0.76rem;
  color: var(--muted);
}

.enemy-cd {
  height: 8px;
}

.enemy-skill {
  font-size: 0.72rem;
  color: var(--muted);
}

.fx-layer {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 9999;
}

.arc-layer {
  width: 100%;
  height: 100%;
}

.attack-arc {
  fill: none;
  stroke-width: 3;
  stroke-linecap: round;
  stroke-dasharray: 12 10;
  animation: arc-fade 420ms ease-out forwards;
}

.damage-popup {
  position: fixed;
  transform: translate(-50%, -50%);
  font-size: 14px;
  font-weight: 900;
  text-shadow: 0 1px 2px rgb(0 0 0 / 0.45);
  animation: damage-float 900ms cubic-bezier(0.15, 0.85, 0.2, 1) forwards;
}

.damage-popup.enemy-hit {
  color: #89ff8d;
}

.damage-popup.player-hit {
  color: #ff6d6d;
}

@keyframes damage-float {
  0% {
    transform: translate(-50%, -42%) scale(0.86);
    opacity: 0;
  }
  18% {
    transform: translate(-50%, -52%) scale(1.26);
    opacity: 1;
  }
  56% {
    transform: translate(-50%, -92%) scale(1.02);
    opacity: 0.9;
  }
  100% {
    transform: translate(-50%, -130%) scale(0.78);
    opacity: 0;
  }
}

@keyframes arc-fade {
  0% {
    opacity: 0.15;
  }
  30% {
    opacity: 1;
  }
  100% {
    opacity: 0;
  }
}

.empty {
  color: var(--muted);
  border: 1px dashed var(--border);
  border-radius: 10px;
  padding: 10px;
  grid-column: 1 / -1;
}

.log-panel {
  margin-top: 12px;
  padding: 10px;
}

.log-panel h2 {
  margin: 0 0 6px;
  font-size: 0.96rem;
}

.logs {
  max-height: 240px;
  overflow: auto;
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 8px;
  background: color-mix(in srgb, var(--surface-2) 86%, transparent);
}

.logs p {
  margin: 0;
  font-size: 0.8rem;
  line-height: 1.4;
}

.logs p + p {
  margin-top: 4px;
}

.muted {
  color: var(--muted);
}

.error {
  margin: 0;
  color: var(--danger);
  font-weight: 800;
}

@media (max-width: 980px) {
  .arena {
    grid-template-columns: 1fr;
  }
}
</style>
