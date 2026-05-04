import { defineStore } from 'pinia'
import { computed, reactive, ref, shallowRef } from 'vue'
import type { RawState } from '@/types/State'
import type {
  GameplayData,
  GameplayEvent,
  GameplayItemPanel,
  GameplayProfileProductionAttribute,
  GameplayProfileBattleAttribute,
  QueueItem,
} from '@/types/GameplayResponse'
import type { Item, Event, ActionsResponse } from '@/types/ActionResponse'
import type { BattleListItem, BattleState } from '@/types/BattleResponse'
import { getJson, apiUrl } from '@/lib/api'
import { applyPatch } from '@/lib/patch'
import { onMessage, onStatusChange } from '@/lib/ws'
import { USE_JSON } from '@/lib/config'
import { StateFull, StateDiff, EquipAction } from '@/pb/state'
import * as actions from '@/lib/actions'
import {
  checkUnlockRequirements,
  buildEventView,
  buildItemPanels,
  buildEquipmentView,
  buildQueue,
  buildActiveLoop,
  buildProfile,
  getLevelProductionMultiplier,
} from '@/lib/gameplayCompute'



export const useGameStore = defineStore('game', () => {
  // ========================================================================
  // ID Registry (new backend numeric ↔ string mapping)
  // ========================================================================
  const idRegistry = ref<{
    items: Record<string, number>
    events: Record<string, number>
    skills: Record<string, number>
    maps: Record<string, number>
  } | null>(null)

  function numToStringId(map: Record<string, number>, numId: number): string | undefined {
    for (const [str, num] of Object.entries(map)) {
      if (num === numId) return str
    }
    return undefined
  }

  // ========================================================================
  // Static data (loaded once at startup)
  // ========================================================================
  const items = ref<Record<string, Item>>({})
  const events = ref<Record<string, Record<string, unknown>>>({})
  const levelProduction = ref<number[]>([])
  const maps = ref<Array<{ id: string; name: string }>>([])
  const staticLoading = ref(false)
  const staticError = ref('')

  // ========================================================================
  // Raw mutable state (delta patches only touch this)
  // ========================================================================
  const state = reactive<RawState>({
    skills: {},
    inventory: [],
    equipment: {},
    tools: {},
    event_counts: {},
    seen_items: [],
    unlocked_events: [],
    attributes: {},
    queue_items: [],
    queue_index: 0,
    queue_progress_seconds: 0,
  })

  // ========================================================================
  // Battle state (managed separately)
  // ========================================================================
  const battleState = shallowRef<BattleState | null>(null)
  const battleEntries = ref<BattleListItem[]>([])
  const battleLoading = ref(false)
  const battleError = ref('')

  // ========================================================================
  // Loading / error
  // ========================================================================
  const stateLoading = ref(false)
  const stateError = ref('')
  const actionError = ref('')
  const actionLoading = ref<Record<string, boolean>>({})

  // ========================================================================
  // Computed views (display layer)
  // ========================================================================
  const itemsMap = computed(() => items.value)

  const productionSkills = computed(() => {
    const order = ['felling', 'mining', 'planting', 'crafting', 'forging', 'enhancing']
    const out: GameplayData['production_skills'] = []
    for (const id of order) {
      const sk = state.skills[id]
      const level = sk ? Math.max(1, Math.floor(sk.level || 1)) : 1
      const exp = sk ? Math.max(0, sk.exp || 0) : 0
      const prevNeed =
        level <= 1 ? 0 : levelProduction.value[Math.max(0, level - 2)] || 0
      const nextNeed =
        level - 1 < levelProduction.value.length
          ? levelProduction.value[level - 1] || prevNeed
          : prevNeed
      const progress =
        nextNeed <= prevNeed ? 1 : Math.max(0, Math.min(1, (exp - prevNeed) / (nextNeed - prevNeed)))
      out.push({
        id,
        name: SKILL_NAME_MAP[id] || id,
        level,
        exp,
        level_progress: progress,
        current_level_total_exp: prevNeed,
        next_level_total_exp: nextNeed,
        level_production_multiplier:
          state.attributes[`${id}_level_production_multiplier`] ?? 1.0,
      })
    }
    return out
  })

  const combatSkills = computed(() => {
    const order = ['strength', 'ranging', 'resilience', 'stamina', 'intelligence', 'defense', 'magic']
    const out: GameplayData['combat_skills'] = []
    for (const id of order) {
      const sk = state.skills[id]
      const level = sk ? Math.max(1, Math.floor(sk.level || 1)) : 1
      const exp = sk ? Math.max(0, sk.exp || 0) : 0
      const prevNeed =
        level <= 1 ? 0 : levelProduction.value[Math.max(0, level - 2)] || 0
      const nextNeed =
        level - 1 < levelProduction.value.length
          ? levelProduction.value[level - 1] || prevNeed
          : prevNeed
      const progress =
        nextNeed <= prevNeed ? 1 : Math.max(0, Math.min(1, (exp - prevNeed) / (nextNeed - prevNeed)))
      out.push({
        id,
        name: SKILL_NAME_MAP[id] || id,
        level,
        exp,
        level_progress: progress,
        current_level_total_exp: prevNeed,
        next_level_total_exp: nextNeed,
        level_production_multiplier: 1.0,
      })
    }
    return out
  })

  const loopEvents = computed(() => {
    const out: GameplayEvent[] = []
    for (const event of Object.values(events.value)) {
      if (event.type !== 'loop') continue
      if (!checkUnlockRequirements(state, event.requirements as any)) continue
      out.push(buildEventView(event, state as any, itemsMap.value))
    }
    return out
  })

  const upgradeEvents = computed(() => {
    const out: GameplayEvent[] = []
    for (const event of Object.values(events.value)) {
      if (event.type !== 'upgrade') continue
      if (!checkUnlockRequirements(state, event.requirements as any)) continue
      const view = buildEventView(event, state as any, itemsMap.value)
      if (view.event_count >= (view.max_executions ?? 1)) continue
      out.push(view)
    }
    return out
  })

  const itemPanels = computed(() => buildItemPanels(state as any, itemsMap.value))

  const equipmentView = computed(() => buildEquipmentView(state as any, itemsMap.value))

  const queue = computed(() => buildQueue(state as any, events.value))

  const activeLoop = computed(() => buildActiveLoop(state as any, events.value))

  const profile = computed(() => buildProfile(state as any, itemsMap.value, levelProduction.value))

  const battleRunning = computed(() => {
    const s = battleState.value
    return s != null && s.status !== 'stopped'
  })

  const activeBattleId = computed(() => battleState.value?.battle_id || '')

  // ========================================================================
  // Helpers
  // ========================================================================
  function setActionLoading(key: string, loading: boolean) {
    actionLoading.value[key] = loading
  }

  function isActionLoading(key: string) {
    return !!actionLoading.value[key]
  }

  function clearActionError() {
    actionError.value = ''
  }

  function resetState() {
    state.skills = {}
    state.inventory = []
    state.equipment = {}
    state.tools = {}
    state.event_counts = {}
    state.seen_items = []
    state.unlocked_events = []
    state.attributes = {}
    state.queue_items = []
    state.queue_index = 0
    state.queue_progress_seconds = 0
    battleState.value = null
    battleEntries.value = []
    battleLoading.value = false
    battleError.value = ''
    stateLoading.value = false
    stateError.value = ''
    actionError.value = ''
    actionLoading.value = {}
  }

  // ========================================================================
  // Data loading
  // ========================================================================
  async function fetchStaticData() {
    staticLoading.value = true
    staticError.value = ''
    try {
      const res = await getJson<{
        actions: { items: Item[]; events: Event[] }
        id_registry: { items: Record<string, number>; events: Record<string, number>; skills: Record<string, number>; maps: Record<string, number> }
        level_curve_csv: string
      }>('/api/v1/game/config')
      if (!res.ok) {
        staticError.value = res.error
        return
      }
      const data = res.data

      // Parse items
      const rawItems: Item[] = data.actions.items || []
      const rawEvents: Event[] = data.actions.events || []
      const itemMap: Record<string, Item> = {}
      for (const item of rawItems) {
        itemMap[item.id] = item
      }
      const eventMap: Record<string, Record<string, unknown>> = {}
      for (const event of rawEvents) {
        eventMap[event.id] = event as unknown as Record<string, unknown>
      }
      items.value = itemMap
      events.value = eventMap

      // Parse id registry
      idRegistry.value = data.id_registry
      actions.setActionIdRegistry(data.id_registry)

      // Parse level curve from CSV
      const csv = data.level_curve_csv || ''
      const lines = csv.trim().split(/\r?\n/)
      const curve: number[] = []
      for (let i = 1; i < lines.length; i++) {
        const line = lines[i]
        if (!line) continue
        const parts = line.split(',')
        if (parts.length >= 2) {
          const v = parseFloat(parts[1]!.trim())
          if (!isNaN(v)) curve.push(v)
        }
      }
      levelProduction.value = curve

      // Build map list
      const seenMaps = new Set<string>()
      maps.value = []
      for (const event of rawEvents) {
        const mapId = event.map || 'unknown'
        if (seenMaps.has(mapId)) continue
        seenMaps.add(mapId)
        maps.value.push({
          id: mapId,
          name: MAP_NAME_MAP[mapId] || mapId,
        })
      }
    } catch (err: any) {
      staticError.value = err.message || '加载静态数据失败'
    } finally {
      staticLoading.value = false
    }
  }

  async function fetchGameplayData() {
    // New backend pushes initial state via WS state.full on connect.
    // No explicit HTTP fetch is needed.
    stateLoading.value = true
    stateError.value = ''
    // Give a brief moment for state.full to arrive if not yet received
    await new Promise((r) => setTimeout(r, 300))
    stateLoading.value = false
  }

  // ========================================================================
  // State application (new backend format → old RawState)
  // ========================================================================
  function applyStateFull(full: StateFull) {
    if (!idRegistry.value) return

    // Skills
    state.skills = {}
    for (const sk of full.skillXp ?? []) {
      const strId = numToStringId(idRegistry.value.skills, sk.skillId ?? 0)
      if (strId) {
        state.skills[strId] = { level: sk.level ?? 0, exp: sk.xp ?? 0 }
      }
    }

    // Inventory
    state.inventory = []
    for (const it of full.inventory ?? []) {
      const strId = numToStringId(idRegistry.value.items, it.itemId ?? 0)
      if (strId && (it.quantity ?? 0) > 0) {
        state.inventory.push({ id: strId, state: it.itemState ?? 0, qty: it.quantity ?? 0 })
      }
    }

    // Equipment / Tools
    state.equipment = {}
    state.tools = {}
    const toolSlots = new Set(['felling', 'mining', 'planting', 'crafting', 'forging', 'enhancing'])
    for (const [slot, it] of Object.entries(full.equipment ?? {})) {
      const strId = numToStringId(idRegistry.value.items, it?.itemId ?? 0)
      if (strId) {
        if (toolSlots.has(slot)) {
          state.tools[slot] = strId
        } else {
          state.equipment[slot] = strId
        }
      }
    }

    // Bestiary → unlocked_events
    state.unlocked_events = []
    for (const b of full.bestiary ?? []) {
      if (b.type === 'event') {
        const strId = numToStringId(idRegistry.value.events, parseInt(b.id ?? '', 10))
        if (strId) state.unlocked_events.push(strId)
      }
    }

    // Attributes
    state.attributes = {}
    for (const attr of full.attribute ?? []) {
      state.attributes[attr.attrId ?? ''] = attr.finalValue ?? 0
    }

    // Queue
    state.queue_items = []
    for (const ex of full.eventExecution ?? []) {
      const strId = numToStringId(idRegistry.value.events, ex.eventId ?? 0)
      if (strId) {
        state.queue_items.push({
          event_id: strId,
          iterations: (ex.targetCycles ?? 0) > 0 ? ex.targetCycles ?? 0 : null,
          completed: 0,
        })
      }
    }
    state.queue_index = 0
    state.queue_progress_seconds = full.eventExecution?.[0]?.progress ?? 0
  }

  function applyStateDiff(diff: StateDiff) {
    if (!idRegistry.value) return

    if (diff.inventory) {
      for (const d of diff.inventory) {
        const strId = numToStringId(idRegistry.value.items, d.itemId ?? 0)
        if (!strId) continue
        const idx = state.inventory.findIndex((e) => e.id === strId && (e.state ?? 0) === (d.itemState ?? 0))
        if (idx >= 0) {
          const entry = state.inventory[idx]!
          const nextQty = entry.qty + (d.quantityDelta ?? 0)
          if (nextQty <= 0) {
            state.inventory.splice(idx, 1)
          } else {
            state.inventory[idx] = { id: entry.id, state: entry.state, qty: nextQty }
          }
        } else if ((d.quantityDelta ?? 0) > 0) {
          state.inventory.push({ id: strId, state: d.itemState ?? 0, qty: d.quantityDelta ?? 0 })
        }
      }
    }

    if (diff.skillXp) {
      for (const d of diff.skillXp) {
        const strId = numToStringId(idRegistry.value.skills, d.skillId ?? 0)
        if (strId) {
          const existing = state.skills[strId]
          state.skills[strId] = {
            level: d.newLevel ?? 0,
            exp: existing ? existing.exp + (d.xpDelta ?? 0) : (d.xpDelta ?? 0),
          }
        }
      }
    }

    if (diff.attribute) {
      for (const d of diff.attribute) {
        state.attributes[d.attrId ?? ''] = d.finalValue ?? 0
      }
    }

    if (diff.bestiary) {
      const existing = new Set(state.unlocked_events)
      for (const b of diff.bestiary) {
        if (b.type === 'event') {
          const strId = numToStringId(idRegistry.value.events, parseInt(b.id ?? '', 10))
          if (strId) existing.add(strId)
        }
      }
      state.unlocked_events = Array.from(existing)
    }

    if (diff.eventQueue) {
      for (const qd of diff.eventQueue) {
        if (qd.scope === 'full') {
          state.queue_items = []
          for (const e of qd.entries ?? []) {
            const strId = numToStringId(idRegistry.value.events, e.eventId ?? 0)
            if (strId) {
              state.queue_items.push({
                event_id: strId,
                iterations: (e.targetCycles ?? 0) > 0 ? e.targetCycles ?? 0 : null,
                completed: 0,
              })
            }
          }
        } else if (qd.scope === 'current' && (qd.entries?.length ?? 0) > 0) {
          state.queue_progress_seconds = qd.entries![0]!.progress ?? 0
        }
      }
    }

    if (diff.equipment) {
      const toolSlots = new Set(['felling', 'mining', 'planting', 'crafting', 'forging', 'enhancing'])
      for (const d of diff.equipment) {
        const isUnequip = d.action === EquipAction.EQUIP_ACTION_UNEQUIP || (d.itemId ?? 0) === 0
        if (isUnequip) {
          if (toolSlots.has(d.slot ?? '')) {
            delete state.tools[d.slot ?? '']
          } else {
            delete state.equipment[d.slot ?? '']
          }
        } else {
          const strId = numToStringId(idRegistry.value.items, d.itemId ?? 0)
          if (strId) {
            if (toolSlots.has(d.slot ?? '')) {
              state.tools[d.slot ?? ''] = strId
            } else {
              state.equipment[d.slot ?? ''] = strId
            }
          }
        }
      }
    }
  }

  // Legacy delta patch (kept for compat, not used with new backend)
  function applyDelta(patch: Record<string, unknown>) {
    applyPatch(state, patch)
  }

  // ========================================================================
  // Battle
  // ========================================================================
  async function fetchBattleList() {
    try {
      battleEntries.value = await actions.fetchBattleList()
    } catch {
      battleEntries.value = []
    }
  }

  async function syncBattleState() {
    try {
      battleState.value = await actions.syncBattleState()
    } catch {
      battleState.value = null
    }
  }

  async function startBattle(battleId: string) {
    battleLoading.value = true
    battleError.value = ''
    try {
      battleState.value = await actions.startBattle(battleId)
      await syncAndSettle()
    } catch (e: any) {
      battleError.value = e.message || '请求失败'
    } finally {
      battleLoading.value = false
    }
  }

  async function stopBattle() {
    battleLoading.value = true
    battleError.value = ''
    try {
      battleState.value = await actions.stopBattle()
      await syncAndSettle()
    } catch (e: any) {
      battleError.value = e.message || '请求失败'
    } finally {
      battleLoading.value = false
    }
  }

  // ========================================================================
  // Player actions
  // ========================================================================
  async function syncAndSettle() {
    const patch = await actions.syncAndSettle()
    if (patch) {
      applyDelta(patch)
    }
  }

  async function startLoop(eventId: string, iterations?: number) {
    actionError.value = ''
    setActionLoading('loop', true)
    try {
      await actions.startLoop(eventId, iterations)
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('loop', false)
    }
  }

  async function stopLoop() {
    actionError.value = ''
    setActionLoading('loop', true)
    try {
      await actions.stopLoop()
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('loop', false)
    }
  }

  async function queueAppend(eventId: string, iterations?: number) {
    actionError.value = ''
    setActionLoading('queue', true)
    try {
      await actions.queueAppend(eventId, iterations)
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('queue', false)
    }
  }

  async function queueRemove(index: number) {
    actionError.value = ''
    setActionLoading('queue', true)
    try {
      await actions.queueRemove(index)
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('queue', false)
    }
  }

  async function queueSwap(fromIndex: number, toIndex: number) {
    actionError.value = ''
    setActionLoading('queue', true)
    try {
      await actions.queueSwap(fromIndex, toIndex)
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('queue', false)
    }
  }

  async function queueBringToFront(index: number) {
    actionError.value = ''
    setActionLoading('queue', true)
    try {
      await actions.queueBringToFront(index)
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('queue', false)
    }
  }

  async function executeInstant(eventId: string) {
    actionError.value = ''
    setActionLoading('instant', true)
    try {
      await actions.executeInstant(eventId)
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('instant', false)
    }
  }

  async function executeUpgrade(eventId: string) {
    actionError.value = ''
    setActionLoading('upgrade', true)
    try {
      await actions.executeUpgrade(eventId)
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('upgrade', false)
    }
  }

  async function equipItem(itemId: string, slot: string) {
    actionError.value = ''
    setActionLoading('equip', true)
    try {
      await actions.equipItem(itemId, slot)
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('equip', false)
    }
  }

  async function unequipItem(slot: string) {
    actionError.value = ''
    setActionLoading('equip', true)
    try {
      await actions.unequipItem(slot)
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('equip', false)
    }
  }

  // ========================================================================
  // WS subscriptions (initiated once)
  // ========================================================================
  let unsubFull: (() => void) | null = null
  let unsubDelta: (() => void) | null = null
  let unsubError: (() => void) | null = null
  let unsubStatus: (() => void) | null = null

  function initWsListeners() {
    if (unsubFull) return // already initialized

    unsubFull = onMessage('state.full', (_type, payload) => {
      try {
        const full = USE_JSON ? StateFull.fromJSON(payload) : StateFull.decode(payload as Uint8Array)
        applyStateFull(full)
      } catch (e) {
        console.error('Failed to decode state.full', e)
      }
    })

    unsubDelta = onMessage('state.diff', (_type, payload) => {
      try {
        const diff = USE_JSON ? StateDiff.fromJSON(payload) : StateDiff.decode(payload as Uint8Array)
        applyStateDiff(diff)
      } catch (e) {
        console.error('Failed to decode state.diff', e)
      }
    })

    unsubError = onMessage('error', (_type, _payload, envelope) => {
      actionError.value = String(envelope.error?.message || '服务器错误')
    })

    unsubStatus = onStatusChange((status) => {
      if (status === 'open') {
        // Battle stubs disabled until backend implements battle handlers
        // syncBattleState().catch((e) => console.warn('syncBattleState failed:', e))
        // fetchBattleList().catch((e) => console.warn('fetchBattleList failed:', e))
      }
    })
  }

  function disposeWsListeners() {
    unsubFull?.()
    unsubDelta?.()
    unsubError?.()
    unsubStatus?.()
    unsubFull = null
    unsubDelta = null
    unsubError = null
    unsubStatus = null
  }

  // ========================================================================
  // Return
  // ========================================================================
  return {
    // Static
    items,
    events,
    levelProduction,
    maps,
    staticLoading,
    staticError,
    // Raw state
    state,
    // Battle
    battleState,
    battleEntries,
    battleLoading,
    battleError,
    battleRunning,
    activeBattleId,
    // Loading / error
    stateLoading,
    stateError,
    actionError,
    actionLoading,
    isActionLoading,
    clearActionError,
    resetState,
    // Computed views
    productionSkills,
    combatSkills,
    loopEvents,
    upgradeEvents,
    itemPanels,
    equipmentView,
    queue,
    activeLoop,
    profile,
    // Data loading
    fetchStaticData,
    fetchGameplayData,
    applyDelta,
    // Battle
    fetchBattleList,
    syncBattleState,
    startBattle,
    stopBattle,
    // Actions
    syncAndSettle,
    startLoop,
    stopLoop,
    queueAppend,
    queueRemove,
    queueSwap,
    queueBringToFront,
    executeInstant,
    executeUpgrade,
    equipItem,
    unequipItem,
    // WS
    initWsListeners,
    disposeWsListeners,
  }
})

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------
const SKILL_NAME_MAP: Record<string, string> = {
  felling: '砍伐',
  mining: '采矿',
  planting: '种植',
  crafting: '制造',
  forging: '锻造',
  enhancing: '赋能',
  trading: '贸易',
  strength: '力量',
  ranging: '远程',
  resilience: '坚韧',
  stamina: '耐力',
  intelligence: '智力',
  defense: '防御',
  magic: '魔法',
  none: '通用',
}

const MAP_NAME_MAP: Record<string, string> = {
  village: '村庄',
}
