import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import type {
  GameConfig,
  ItemDef,
  EventDef,
  StateFull,
  StateDiff,
  InventoryDiff,
  AttributeDiff,
  SkillXPDiff,
  BestiaryDiff,
  EventQueueDiff,
  EquipmentDiff,
  GameplaySkill,
  GameplayEvent,
  GameplayItemPanel,
  QueueItem,
  GameplayData,
  EventExecution,
} from '@/types/game'
import { getJson, apiUrl } from '@/lib/api'
import { onMessage, send, onStatusChange } from '@/lib/ws'

function parseCSV(csv: string): number[] {
  const lines = csv.trim().split(/\r?\n/)
  const out: number[] = []
  for (let i = 1; i < lines.length; i++) {
    const line = lines[i].trim()
    if (!line) continue
    const parts = line.split(',')
    if (parts.length >= 2) {
      const v = parseFloat(parts[1].trim())
      if (!isNaN(v)) out.push(v)
    }
  }
  return out
}

export const useGameStore = defineStore('game', () => {
  // ========================================================================
  // Static data
  // ========================================================================
  const config = ref<GameConfig | null>(null)
  const itemsById = ref<Map<number, ItemDef>>(new Map())
  const eventsById = ref<Map<number, EventDef>>(new Map())
  const idToItem = ref<Map<number, string>>(new Map())
  const idToEvent = ref<Map<number, string>>(new Map())
  const idToSkill = ref<Map<number, string>>(new Map())
  const levelCurve = ref<number[]>([])
  const staticLoading = ref(false)
  const staticError = ref('')

  // ========================================================================
  // Raw mutable state
  // ========================================================================
  const inventory = reactive<Map<string, number>>(new Map()) // key: `${item_id}:${item_state}`
  const attributes = reactive<Map<string, AttributeDiff>>(new Map())
  const skills = reactive<Map<number, { level: number; xp: number }>>(new Map())
  const bestiary = reactive<Set<string>>(new Set()) // "type:id"
  const eventQueue = reactive<EventExecution[]>([])
  const equipment = reactive<Map<string, { item_id: number; item_state: number }>>(new Map())

  // ========================================================================
  // Loading / error
  // ========================================================================
  const stateLoading = ref(false)
  const stateError = ref('')
  const actionError = ref('')
  const actionLoading = ref<Record<string, boolean>>({})

  // ========================================================================
  // Computed helpers
  // ========================================================================
  const itemStringByNum = computed(() => {
    const map = new Map<number, string>()
    for (const [strId, numId] of Object.entries(config.value?.id_registry.items ?? {})) {
      map.set(numId, strId)
    }
    return map
  })

  const eventStringByNum = computed(() => {
    const map = new Map<number, string>()
    for (const [strId, numId] of Object.entries(config.value?.id_registry.events ?? {})) {
      map.set(numId, strId)
    }
    return map
  })

  const skillStringByNum = computed(() => {
    const map = new Map<number, string>()
    for (const [strId, numId] of Object.entries(config.value?.id_registry.skills ?? {})) {
      map.set(numId, strId)
    }
    return map
  })

  const mapNameById = computed(() => {
    const map: Record<string, string> = {}
    for (const [strId, numId] of Object.entries(config.value?.id_registry.maps ?? {})) {
      map[strId] = strId
    }
    return map
  })

  const itemByNumId = (numId: number): ItemDef | undefined => {
    const strId = itemStringByNum.value.get(numId)
    if (!strId) return undefined
    return config.value?.actions.items.find((i) => i.id === strId)
  }

  const eventByNumId = (numId: number): EventDef | undefined => {
    const strId = eventStringByNum.value.get(numId)
    if (!strId) return undefined
    return config.value?.actions.events.find((e) => e.id === strId)
  }

  // ========================================================================
  // Computed views
  // ========================================================================
  const productionSkills = computed((): GameplaySkill[] => {
    const order = ['felling', 'mining', 'planting', 'crafting', 'forging', 'enhancing']
    const out: GameplaySkill[] = []
    for (const id of order) {
      const numId = config.value?.id_registry.skills?.[id]
      const sk = numId !== undefined ? skills.get(numId) : undefined
      const level = sk ? Math.max(1, Math.floor(sk.level || 1)) : 1
      const exp = sk ? Math.max(0, sk.xp || 0) : 0
      const prevNeed = level <= 1 ? 0 : levelCurve.value[Math.max(0, level - 2)] || 0
      const nextNeed = level - 1 < levelCurve.value.length ? levelCurve.value[level - 1] || prevNeed : prevNeed
      const progress = nextNeed <= prevNeed ? 1 : Math.max(0, Math.min(1, (exp - prevNeed) / (nextNeed - prevNeed)))
      const attrName = `${id}_production_multiplier`
      const attrVal = attributes.get(attrName)?.final_value ?? 1.0
      out.push({
        id,
        name: SKILL_NAME_MAP[id] || id,
        level,
        exp,
        level_progress: progress,
        current_level_total_exp: prevNeed,
        next_level_total_exp: nextNeed,
        level_production_multiplier: attrVal,
      })
    }
    return out
  })

  const combatSkills = computed((): GameplaySkill[] => {
    const order = ['strength', 'ranging', 'resilience', 'stamina', 'intelligence', 'defense', 'magic']
    const out: GameplaySkill[] = []
    for (const id of order) {
      const numId = config.value?.id_registry.skills?.[id]
      const sk = numId !== undefined ? skills.get(numId) : undefined
      const level = sk ? Math.max(1, Math.floor(sk.level || 1)) : 1
      const exp = sk ? Math.max(0, sk.xp || 0) : 0
      const prevNeed = level <= 1 ? 0 : levelCurve.value[Math.max(0, level - 2)] || 0
      const nextNeed = level - 1 < levelCurve.value.length ? levelCurve.value[level - 1] || prevNeed : prevNeed
      const progress = nextNeed <= prevNeed ? 1 : Math.max(0, Math.min(1, (exp - prevNeed) / (nextNeed - prevNeed)))
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

  const allEvents = computed(() => config.value?.actions.events ?? [])

  const unlockedEventIds = computed(() => {
    const ids = new Set<string>()
    for (const key of bestiary) {
      if (key.startsWith('event:')) {
        ids.add(key.slice(6))
      }
    }
    return ids
  })

  const loopEvents = computed((): GameplayEvent[] => {
    const out: GameplayEvent[] = []
    if (!config.value) return out
    for (const event of config.value.actions.events) {
      if (event.type !== 'loop') continue
      if (!checkUnlockRequirements(event)) continue
      out.push(buildEventView(event))
    }
    return out
  })

  const upgradeEvents = computed((): GameplayEvent[] => {
    const out: GameplayEvent[] = []
    if (!config.value) return out
    for (const event of config.value.actions.events) {
      if (event.type !== 'upgrade') continue
      if (!checkUnlockRequirements(event)) continue
      const view = buildEventView(event)
      // For upgrade events, we don't track counts locally in this version
      out.push(view)
    }
    return out
  })

  const itemPanels = computed((): GameplayItemPanel[] => {
    const panels = new Map<string, GameplayItemPanel['items']>()
    for (const [key, qty] of inventory) {
      if (qty <= 0) continue
      const [itemIdStr] = key.split(':')
      const itemId = parseInt(itemIdStr, 10)
      const def = itemByNumId(itemId)
      if (!def) continue
      const cls = def.classification || '其他'
      if (!panels.has(cls)) panels.set(cls, [])
      panels.get(cls)!.push({ id: def.id, name: def.name, quantity: qty })
    }
    const out: GameplayItemPanel[] = []
    for (const [classification, items] of panels) {
      items.sort((a, b) => a.name.localeCompare(b.name))
      out.push({ classification, title: CLASS_NAME_MAP[classification] || classification, items })
    }
    out.sort((a, b) => a.title.localeCompare(b.title))
    return out
  })

  const queue = computed((): { items: QueueItem[]; index: number; progress_seconds: number } => {
    const items: QueueItem[] = []
    let index = 0
    let progress = 0
    for (let i = 0; i < eventQueue.length; i++) {
      const entry = eventQueue[i]
      const eventDef = eventByNumId(entry.event_id)
      if (!eventDef) continue
      items.push({
        index: i,
        event_id: eventDef.id,
        name: eventDef.name,
        type: eventDef.type,
        map: eventDef.map,
        map_name: MAP_NAME_MAP[eventDef.map] || eventDef.map,
        is_current: i === 0,
        is_executable: true,
        iterations: entry.target_cycles > 0 ? entry.target_cycles : null,
        completed: 0,
        remaining: entry.target_cycles > 0 ? entry.target_cycles : null,
      })
      if (i === 0) {
        progress = entry.progress
      }
    }
    return { items, index, progress_seconds: progress }
  })

  const activeLoop = computed(() => {
    const q = queue.value
    if (q.items.length === 0) return null
    const head = q.items[0]
    if (head.type !== 'loop') return null
    const eventDef = config.value?.actions.events.find((e) => e.id === head.event_id)
    if (!eventDef || !eventDef.loop_time) return null
    const exec = eventQueue[0]
    if (!exec) return null
    return {
      event_id: head.event_id,
      elapsed_seconds: exec.progress,
      duration_seconds: eventDef.loop_time,
      available_iterations: head.iterations,
    }
  })

  const equipmentView = computed(() => {
    const productionSlots: Array<{
      slot_type: 'tool'
      slot_id: string
      slot_name: string
      item_id: string | null
      item_name: string | null
      is_disabled: boolean
      enhance_level: number | null
    }> = []
    const battleSlots: Array<{
      slot_type: 'equipment'
      slot_id: string
      slot_name: string
      item_id: string | null
      item_name: string | null
      is_disabled: boolean
      enhance_level: number | null
    }> = []

    // Tool slots
    const toolOrder = ['felling', 'mining', 'planting', 'crafting', 'forging', 'enhancing']
    for (const slot of toolOrder) {
      const eq = equipment.get(slot)
      productionSlots.push({
        slot_type: 'tool',
        slot_id: slot,
        slot_name: SKILL_NAME_MAP[slot] || slot,
        item_id: eq ? itemStringByNum.value.get(eq.item_id) ?? null : null,
        item_name: eq ? itemByNumId(eq.item_id)?.name ?? null : null,
        is_disabled: false,
        enhance_level: null,
      })
    }

    // Equipment slots
    const equipOrder = ['main_hand', 'off_hand', 'head', 'chest', 'legs', 'feet', 'neck', 'ring1', 'ring2']
    for (const slot of equipOrder) {
      const eq = equipment.get(slot)
      battleSlots.push({
        slot_type: 'equipment',
        slot_id: slot,
        slot_name: EQUIP_SLOT_NAME_MAP[slot] || slot,
        item_id: eq ? itemStringByNum.value.get(eq.item_id) ?? null : null,
        item_name: eq ? itemByNumId(eq.item_id)?.name ?? null : null,
        is_disabled: false,
        enhance_level: null,
      })
    }

    const equipableItems: Array<{ id: string; name: string; quantity: number; slot_type: 'tool' | 'equipment'; required_slots: string[] }> = []
    for (const [key, qty] of inventory) {
      if (qty <= 0) continue
      const [itemIdStr] = key.split(':')
      const itemId = parseInt(itemIdStr, 10)
      const def = itemByNumId(itemId)
      if (!def) continue
      if (def.tool && def.tool_details) {
        for (const req of def.tool_details.tool_position_requirement) {
          equipableItems.push({
            id: def.id,
            name: def.name,
            quantity: qty,
            slot_type: 'tool',
            required_slots: [req.tool_position],
          })
        }
      }
      if (def.equipment && def.equipment_details) {
        equipableItems.push({
          id: def.id,
          name: def.name,
          quantity: qty,
          slot_type: 'equipment',
          required_slots: def.equipment_details.equipment_position_requirements.map((r) => r.position),
        })
      }
    }

    return { production_slots: productionSlots, battle_slots: battleSlots, equipable_items: equipableItems }
  })

  // ========================================================================
  // Helpers
  // ========================================================================
  function checkUnlockRequirements(event: EventDef): boolean {
    for (const req of event.requirements) {
      if (req.type === 'skill') {
        const skillNumId = config.value?.id_registry.skills?.[req.id]
        const sk = skillNumId !== undefined ? skills.get(skillNumId) : undefined
        const level = sk ? sk.level : 0
        const threshold = req.value ?? 0
        const cmp = req.comparison_types || 'bigger_or_equal'
        if (cmp === 'bigger_or_equal' && level < threshold) return false
        if (cmp === 'bigger' && level <= threshold) return false
        if (cmp === 'equal' && level !== threshold) return false
        if (cmp === 'smaller' && level >= threshold) return false
        if (cmp === 'smaller_or_equal' && level > threshold) return false
      } else if (req.type === 'event') {
        if (!unlockedEventIds.value.has(req.id)) return false
      }
    }
    return true
  }

  function buildEventView(event: EventDef): GameplayEvent {
    const requiredSkills: GameplayEvent['required_skills'] = []
    const costItems: GameplayEvent['cost_items'] = []
    const rewardPreview: GameplayEvent['reward_preview'] = []

    for (const req of event.requirements) {
      if (req.type === 'skill' && req.comparison_types) {
        requiredSkills.push({
          skill_id: req.id,
          skill_name: SKILL_NAME_MAP[req.id] || req.id,
          comparison_types: req.comparison_types,
          comparison_text: req.comparison_types.replace(/_/g, ' '),
          value: req.value ?? 0,
        })
      } else if ((req.type === 'item' || req.type === 'fluid') && !req.comparison_types) {
        const def = config.value?.actions.items.find((i) => i.id === req.id)
        costItems.push({ item_id: req.id, item_name: def?.name || req.id, value: req.value ?? 0 })
      }
    }

    for (const rew of event.rewards) {
      if (!rew.type || rew.type === '') {
        const def = config.value?.actions.items.find((i) => i.id === rew.id)
        rewardPreview.push({
          item_id: rew.id || '',
          item_name: def?.name || rew.id || '',
          base_value: rew.num ?? rew.value ?? 0,
          effective_value: rew.num ?? rew.value ?? 0,
        })
      }
    }

    return {
      id: event.id,
      name: event.name,
      description: event.description,
      type: event.type,
      map: event.map,
      map_name: MAP_NAME_MAP[event.map] || event.map,
      need_skill: event.need_skill,
      need_skill_name: SKILL_NAME_MAP[event.need_skill] || event.need_skill,
      loop_time: event.loop_time,
      effective_loop_time: event.loop_time,
      experience: undefined,
      event_count: 0,
      max_executions: event.type === 'upgrade' ? 1 : null,
      required_skills: requiredSkills,
      cost_items: costItems,
      reward_preview: rewardPreview,
      is_executable: checkUnlockRequirements(event),
      is_skill_blocked: false,
    }
  }

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
    inventory.clear()
    attributes.clear()
    skills.clear()
    bestiary.clear()
    eventQueue.length = 0
    equipment.clear()
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
        actions: GameConfig['actions']
        id_registry: GameConfig['id_registry']
        attributes: GameConfig['attributes']
        attr_registry: GameConfig['attr_registry']
        level_curve_csv: string
      }>('/api/v1/game/config')
      if (!res.ok) {
        staticError.value = res.error
        return
      }
      const data = res.data
      config.value = {
        actions: data.actions,
        id_registry: data.id_registry,
        attributes: data.attributes,
        attr_registry: data.attr_registry,
        level_curve_csv: data.level_curve_csv,
      }
      levelCurve.value = parseCSV(data.level_curve_csv)

      const imap = new Map<number, ItemDef>()
      for (const item of data.actions.items) {
        const numId = data.id_registry.items[item.id]
        if (numId !== undefined) imap.set(numId, item)
      }
      itemsById.value = imap

      const emap = new Map<number, EventDef>()
      for (const event of data.actions.events) {
        const numId = data.id_registry.events[event.id]
        if (numId !== undefined) emap.set(numId, event)
      }
      eventsById.value = emap
    } catch (err: any) {
      staticError.value = err.message || '加载静态数据失败'
    } finally {
      staticLoading.value = false
    }
  }

  // ========================================================================
  // State application
  // ========================================================================
  function applyStateFull(full: StateFull) {
    inventory.clear()
    for (const it of full.inventory) {
      inventory.set(`${it.item_id}:${it.item_state}`, it.quantity)
    }
    attributes.clear()
    for (const attr of full.attribute) {
      attributes.set(attr.attr_id, attr)
    }
    skills.clear()
    for (const sk of full.skill_xp) {
      skills.set(sk.skill_id, { level: sk.level, xp: sk.xp })
    }
    bestiary.clear()
    for (const b of full.bestiary) {
      bestiary.add(`${b.type}:${b.id}`)
    }
    eventQueue.length = 0
    for (const ex of full.event_execution) {
      eventQueue.push({
        queue_id: ex.queue_id,
        event_id: ex.event_id,
        target_cycles: ex.target_cycles,
        progress: ex.progress,
      })
    }
    equipment.clear()
    for (const [slot, it] of Object.entries(full.equipment)) {
      equipment.set(slot, { item_id: it.item_id, item_state: it.item_state })
    }
  }

  function applyStateDiff(diff: StateDiff) {
    if (diff.inventory) {
      for (const d of diff.inventory) {
        const key = `${d.item_id}:${d.item_state}`
        const current = inventory.get(key) ?? 0
        const next = current + d.quantity_delta
        if (next <= 0) inventory.delete(key)
        else inventory.set(key, next)
      }
    }
    if (diff.attribute) {
      for (const d of diff.attribute) {
        attributes.set(d.attr_id, d)
      }
    }
    if (diff.skill_xp) {
      for (const d of diff.skill_xp) {
        skills.set(d.skill_id, { level: d.new_level, xp: (skills.get(d.skill_id)?.xp ?? 0) + d.xp_delta })
      }
    }
    if (diff.bestiary) {
      for (const d of diff.bestiary) {
        bestiary.add(`${d.type}:${d.id}`)
      }
    }
    if (diff.event_queue) {
      for (const qd of diff.event_queue) {
        if (qd.scope === 'full') {
          // Replace entire queue entries for this queue_id
          const newEntries: EventExecution[] = []
          for (const e of qd.entries) {
            newEntries.push({
              queue_id: qd.queue_id,
              event_id: e.event_id,
              target_cycles: e.target_cycles,
              progress: e.progress,
            })
          }
          // Remove old entries for this queue_id and add new ones
          for (let i = eventQueue.length - 1; i >= 0; i--) {
            if (eventQueue[i].queue_id === qd.queue_id) {
              eventQueue.splice(i, 1)
            }
          }
          for (const e of newEntries) eventQueue.push(e)
          // Sort by queue_id then position
          eventQueue.sort((a, b) => {
            if (a.queue_id !== b.queue_id) return a.queue_id - b.queue_id
            return 0
          })
        } else {
          // Update progress of current entry
          const entry = eventQueue.find((e) => e.queue_id === qd.queue_id)
          if (entry && qd.entries.length > 0) {
            entry.progress = qd.entries[0].progress
          }
        }
      }
    }
    if (diff.equipment) {
      for (const d of diff.equipment) {
        if (d.action === 'EQUIP_ACTION_UNEQUIP' || d.item_id === 0) {
          equipment.delete(d.slot)
        } else {
          equipment.set(d.slot, { item_id: d.item_id, item_state: d.item_state })
        }
      }
    }
  }

  // ========================================================================
  // Actions
  // ========================================================================
  async function queueAppend(eventId: string, iterations?: number) {
    actionError.value = ''
    setActionLoading('queue', true)
    try {
      const numId = config.value?.id_registry.events?.[eventId]
      if (numId === undefined) throw new Error('未知事件')
      await send('queue.append', {
        event_id: numId,
        target_cycles: iterations ?? -1,
        queue_id: 0,
      })
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
      await send('queue.remove', { position: index, queue_id: 0 })
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('queue', false)
    }
  }

  async function queueMove(fromIndex: number, toIndex: number) {
    actionError.value = ''
    setActionLoading('queue', true)
    try {
      await send('queue.move', { from_position: fromIndex, to_position: toIndex, queue_id: 0 })
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('queue', false)
    }
  }

  async function equipItem(itemId: string, slot: string) {
    actionError.value = ''
    setActionLoading('equip', true)
    try {
      const numId = config.value?.id_registry.items?.[itemId]
      if (numId === undefined) throw new Error('未知物品')
      await send('inventory.equip', { item_id: numId, item_state: 0, slot })
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
      await send('inventory.unequip', { slot })
    } catch (e: any) {
      actionError.value = e.message || '请求失败'
    } finally {
      setActionLoading('equip', false)
    }
  }

  // ========================================================================
  // WS subscriptions
  // ========================================================================
  let unsubDelta: (() => void) | null = null
  let unsubFull: (() => void) | null = null
  let unsubError: (() => void) | null = null

  function initWsListeners() {
    if (unsubDelta) return

    unsubFull = onMessage('state.full', (msg) => {
      const payload = msg.payload as StateFull | undefined
      if (payload) applyStateFull(payload)
    })

    unsubDelta = onMessage('state.diff', (msg) => {
      const payload = msg.payload as StateDiff | undefined
      if (payload) applyStateDiff(payload)
    })

    unsubError = onMessage('error', (msg) => {
      actionError.value = msg.error?.message || '服务器错误'
    })
  }

  function disposeWsListeners() {
    unsubFull?.()
    unsubDelta?.()
    unsubError?.()
    unsubFull = null
    unsubDelta = null
    unsubError = null
  }

  // ========================================================================
  // Return
  // ========================================================================
  return {
    // Static
    config,
    staticLoading,
    staticError,
    // Raw state
    inventory,
    attributes,
    skills,
    bestiary,
    eventQueue,
    equipment,
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
    queue,
    activeLoop,
    equipmentView,
    // Helpers
    itemByNumId,
    eventByNumId,
    itemStringByNum,
    eventStringByNum,
    // Data loading
    fetchStaticData,
    applyStateFull,
    applyStateDiff,
    // Actions
    queueAppend,
    queueRemove,
    queueMove,
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

const CLASS_NAME_MAP: Record<string, string> = {
  resources: '资源',
  equipment: '装备',
  tool: '工具',
  ores: '矿石',
}

const EQUIP_SLOT_NAME_MAP: Record<string, string> = {
  main_hand: '主手',
  off_hand: '副手',
  head: '头部',
  chest: '胸部',
  legs: '腿部',
  feet: '脚部',
  neck: '项链',
  ring1: '戒指1',
  ring2: '戒指2',
}
