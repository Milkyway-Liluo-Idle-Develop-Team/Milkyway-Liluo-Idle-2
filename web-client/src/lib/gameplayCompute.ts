import type {
  GameplayData,
  GameplayEvent,
  GameplayItemPanel,
  GameplayProfileProductionAttribute,
  GameplayProfileBattleAttribute,
} from '@/types/GameplayResponse'
import type { Item } from '@/types/ActionResponse'

type GameplayLikeState = Pick<
  GameplayData,
  | 'skills'
  | 'inventory'
  | 'equipment'
  | 'tools'
  | 'event_counts'
  | 'seen_items'
  | 'unlocked_events'
  | 'attributes'
  | 'queue_items'
  | 'queue_index'
  | 'queue_progress_seconds'
> & {
  queue?: GameplayData['queue']
  equipment_view?: GameplayData['equipment_view']
}

// ---------------------------------------------------------------------------
// Constants (mirroring backend)
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

const CLASSIFICATION_NAME_MAP: Record<string, string> = {
  important: '重要',
  resources: '资源',
  ores: '矿物',
  tool: '工具',
  equipment: '装备',
  fuel: '燃料',
  animal_materials: '动物材料',
}

const CLASSIFICATION_ORDER = [
  'important',
  'resources',
  'ores',
  'tool',
  'equipment',
  'fuel',
  'animal_materials',
]

const COMPARISON_TEXT_MAP: Record<string, string> = {
  bigger: '大于',
  equal: '等于',
  smaller: '小于',
  bigger_or_equal: '大于等于',
  smaller_or_equal: '小于等于',
}

const PRODUCTION_SKILL_ORDER = ['felling', 'mining', 'planting', 'crafting', 'forging', 'enhancing']

const COMBAT_SKILL_ORDER = [
  'strength',
  'ranging',
  'resilience',
  'stamina',
  'intelligence',
  'defense',
  'magic',
]

const TOOL_SLOT_ORDER = ['felling', 'mining', 'planting', 'crafting', 'forging', 'enhancing']
const EQUIPMENT_SLOT_ORDER = [
  'main_hand',
  'side_hand',
  'head',
  'chest',
  'leg',
  'feet',
  'necklace',
  'treasure',
]

const TOOL_SLOT_NAME_MAP: Record<string, string> = {
  felling: '砍伐',
  mining: '采矿',
  planting: '种植',
  crafting: '制造',
  forging: '锻造',
  enhancing: '赋能',
}

const EQUIPMENT_SLOT_NAME_MAP: Record<string, string> = {
  main_hand: '主手',
  side_hand: '副手',
  head: '头部',
  chest: '胸部',
  leg: '腿部',
  feet: '足部',
  necklace: '项链',
  treasure: '珍宝',
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function toFloat(value: unknown, defaultVal = 0): number {
  try {
    const n = Number(value)
    return Number.isFinite(n) ? n : defaultVal
  } catch {
    return defaultVal
  }
}

function compare(actual: number, expected: number, comp: string | null | undefined): boolean {
  if (comp == null) return actual >= expected
  const c = String(comp).toLowerCase()
  if (c === 'bigger') return actual > expected
  if (c === 'equal') return actual === expected
  if (c === 'smaller') return actual < expected
  if (c === 'bigger_or_equal') return actual >= expected
  if (c === 'smaller_or_equal') return actual <= expected
  return actual >= expected
}

function requirementExpectedValue(req: Record<string, unknown>): number {
  if (req.value !== undefined && req.value !== null) return toFloat(req.value, 0)
  if (req.type === 'event') return 1
  return 0
}

function itemQuantity(state: GameplayLikeState, itemId: string | undefined, itemState = 0): number {
  if (!itemId) return 0
  const entry = state.inventory.find((e) => e.id === itemId && (e.state ?? 0) === itemState)
  return toFloat(entry?.qty, 0)
}

function totalItemQuantity(state: GameplayLikeState, itemId: string | undefined): number {
  if (!itemId) return 0
  return state.inventory
    .filter((e) => e.id === itemId)
    .reduce((sum, e) => sum + toFloat(e.qty, 0), 0)
}

function fluidQuantity(state: GameplayLikeState, fluidId: string | undefined): number {
  return itemQuantity(state, fluidId, 0)
}

function eventCount(state: GameplayLikeState, eventId: string | undefined): number {
  if (!eventId) return 0
  // Fallback to unlocked_events because backend does not yet persist event_counts.
  if (state.unlocked_events?.includes(eventId)) return 1
  return toFloat(state.event_counts[eventId], 0)
}

function skillLevel(state: GameplayLikeState, skillId: string | undefined): number {
  if (!skillId) return 0
  const sk = state.skills[skillId]
  return sk ? Math.max(0, Math.floor(toFloat(sk.level, 0))) : 0
}

export function checkRequirements(
  state: GameplayLikeState,
  reqs: Array<Record<string, unknown>> | undefined | null,
): boolean {
  if (!reqs) return true
  for (const req of reqs) {
    const rtype = String(req.type || '')
    const rid = String(req.id || '')
    const value = requirementExpectedValue(req)
    const comp = req.comparison_types as string | undefined

    let actual = 0
    if (rtype === 'skill') {
      actual = skillLevel(state, rid)
    } else if (rtype === 'item') {
      if (comp == null && value <= 0) {
        actual = state.seen_items.includes(rid) ? 1 : 0
      } else {
        actual = itemQuantity(state, rid)
      }
    } else if (rtype === 'event') {
      actual = eventCount(state, rid)
    } else if (rtype === 'fluid') {
      actual = fluidQuantity(state, rid)
    } else {
      continue
    }

    if (!compare(actual, value, comp)) return false
  }
  return true
}

export function checkUnlockRequirements(
  state: GameplayLikeState,
  reqs: Array<Record<string, unknown>> | undefined | null,
): boolean {
  if (!reqs) return true
  for (const req of reqs) {
    const rtype = String(req.type || '')
    if (rtype === 'skill') continue
    const rid = String(req.id || '')
    let value = requirementExpectedValue(req)
    const comp = req.comparison_types as string | undefined

    let actual = 0
    if (rtype === 'item') {
      if (comp == null) {
        actual = state.seen_items.includes(rid) ? 1 : 0
        value = 1
      } else {
        actual = itemQuantity(state, rid)
      }
    } else if (rtype === 'event') {
      actual = eventCount(state, rid)
    } else if (rtype === 'fluid') {
      actual = fluidQuantity(state, rid)
    } else {
      continue
    }

    if (!compare(actual, value, comp)) return false
  }
  return true
}

export function isSkillBlocked(
  state: GameplayLikeState,
  reqs: Array<Record<string, unknown>> | undefined | null,
): boolean {
  if (!reqs) return false
  for (const req of reqs) {
    if (req.type !== 'skill') continue
    const rid = String(req.id || '')
    const value = requirementExpectedValue(req)
    const comp = req.comparison_types as string | undefined
    if (!compare(skillLevel(state, rid), value, comp)) return true
  }
  return false
}

export function effectiveLoopTime(
  event: Record<string, unknown>,
  attributes: Record<string, number>,
): number {
  let base = toFloat(event.loop_time, 1)
  if (base <= 0) base = 1

  if (event.combat) {
    const enemy = (event.enemy || {}) as Record<string, number>
    const result = simulateCombat(attributes, enemy)
    return Math.max(result.duration, 0.1)
  }

  const skillId = String(event.need_skill || 'none')
  if (skillId && skillId !== 'none') {
    const speedMult = toFloat(attributes[`${skillId}_speed_multiplier`], 0)
    if (speedMult > 0) {
      base = base / (1 + speedMult)
    }
  }
  return Math.max(base, 0.1)
}

interface CombatResult {
  victory: boolean
  duration: number
}

function simulateCombat(
  attributes: Record<string, number>,
  enemy: Record<string, number>,
): CombatResult {
  const playerAtk = toFloat(attributes.physical_damage, 1)
  const playerDef = toFloat(attributes.defense, 0)
  const playerInterval = toFloat(attributes.attack_interval, 2)

  const enemyHp = toFloat(enemy.hp, 50)
  const enemyAtk = toFloat(enemy.attack, 5)
  const enemyDef = toFloat(enemy.defense, 0)

  const playerDmg = Math.max(1, playerAtk - enemyDef)
  const hitsToKill = Math.ceil(enemyHp / playerDmg)
  const killTime = hitsToKill * playerInterval

  return { victory: true, duration: Math.max(killTime, playerInterval) }
}

export function rewardPreview(
  event: Record<string, unknown>,
  attributes: Record<string, number>,
  itemsMap: Record<string, Item>,
): Array<{ item_id: string; item_name: string; base_value: number; effective_value: number }> {
  const rewards = (event.rewards || []) as Array<Record<string, unknown>>
  if (!rewards.length) return []

  const skillId = String(event.need_skill || 'none')
  const skillMult = skillId === 'none' ? 0 : toFloat(attributes[`${skillId}_reward_mult`], 0)
  const skillFlat = skillId === 'none' ? 0 : toFloat(attributes[`${skillId}_reward_flat`], 0)
  const commonMult = toFloat(attributes.reward_mult, 0)
  const commonFlat = toFloat(attributes.reward_flat, 0)

  const out: Array<{
    item_id: string
    item_name: string
    base_value: number
    effective_value: number
  }> = []
  for (const rew of rewards) {
    const rewardType = String(rew.type || 'item').toLowerCase()
    if (rewardType !== 'item') continue
    const itemId = String(rew.id || '')
    if (!itemId) continue
    const baseRaw = rew.num !== undefined ? rew.num : rew.value
    const baseValue = toFloat(baseRaw, 0)
    if (baseValue <= 0) continue

    const effectiveValue = baseValue * (1 + skillMult) * (1 + commonMult) + skillFlat + commonFlat
    out.push({
      item_id: itemId,
      item_name: itemsMap[itemId]?.name || itemId,
      base_value: baseValue,
      effective_value: Math.max(0, effectiveValue),
    })
  }
  return out
}

export function displayExperience(event: Record<string, unknown>): number {
  const rewards = (event.rewards || []) as Array<Record<string, unknown>>
  let total = 0
  for (const rew of rewards) {
    if (String(rew.type || '').toLowerCase() === 'experience') {
      total += toFloat(rew.value ?? rew.num, 0)
    }
  }
  if (total <= 0) total = toFloat(event.experience, 0)
  return total
}

export function upgradeMaxExecutions(event: Record<string, unknown>): number {
  const raw = event.repeat_time ?? event.loop_time
  if (raw == null) return 1
  const n = Number(raw)
  return Number.isFinite(n) ? Math.max(1, Math.floor(n)) : 1
}

function extractRequiredSkills(reqs: Array<Record<string, unknown>> | undefined | null): Array<{
  skill_id: string
  skill_name: string
  comparison_types: string
  comparison_text: string
  value: number
}> {
  const out: Array<{
    skill_id: string
    skill_name: string
    comparison_types: string
    comparison_text: string
    value: number
  }> = []
  for (const req of reqs || []) {
    if (req.type !== 'skill') continue
    const skillId = String(req.id || '')
    const comp = String(req.comparison_types || 'bigger_or_equal')
    out.push({
      skill_id: skillId,
      skill_name: SKILL_NAME_MAP[skillId] || skillId,
      comparison_types: comp,
      comparison_text: COMPARISON_TEXT_MAP[comp] || comp,
      value: toFloat(req.value, 0),
    })
  }
  return out
}

function extractCostItems(
  reqs: Array<Record<string, unknown>> | undefined | null,
  itemsMap: Record<string, Item>,
): Array<{ item_id: string; item_name: string; value: number }> {
  const out: Array<{ item_id: string; item_name: string; value: number }> = []
  for (const req of reqs || []) {
    if (req.type !== 'item') continue
    if (req.comparison_types !== undefined && req.comparison_types !== null) continue
    const value = toFloat(req.value, 0)
    if (value <= 0) continue
    const itemId = String(req.id || '')
    out.push({
      item_id: itemId,
      item_name: itemsMap[itemId]?.name || itemId,
      value: Math.floor(value),
    })
  }
  return out
}

export function buildEventView(
  event: Record<string, unknown>,
  state: GameplayLikeState,
  itemsMap: Record<string, Item>,
): GameplayEvent {
  const requirements = (event.requirements || []) as Array<Record<string, unknown>>
  const eventId = String(event.id || '')
  const baseLoopTime = toFloat(event.loop_time, 0)
  const etype = String(event.type || '')

  return {
    id: eventId,
    name: String(event.name || eventId),
    description: String(event.description || ''),
    type: etype as 'loop' | 'upgrade' | string,
    map: String(event.map || 'unknown'),
    map_name: MAP_NAME_MAP[String(event.map || 'unknown')] || String(event.map || 'unknown'),
    need_skill: String(event.need_skill || 'none'),
    need_skill_name:
      SKILL_NAME_MAP[String(event.need_skill || 'none')] || String(event.need_skill || 'none'),
    loop_time: baseLoopTime > 0 ? baseLoopTime : undefined,
    effective_loop_time: etype === 'loop' ? effectiveLoopTime(event, state.attributes) : null,
    experience: displayExperience(event),
    event_count: state.unlocked_events?.includes(eventId)
      ? 1
      : toFloat(state.event_counts[eventId], 0),
    max_executions: etype === 'upgrade' ? upgradeMaxExecutions(event) : null,
    required_skills: extractRequiredSkills(requirements),
    cost_items: extractCostItems(requirements, itemsMap),
    reward_preview: rewardPreview(event, state.attributes, itemsMap),
    is_executable: checkRequirements(state, requirements),
    is_skill_blocked: isSkillBlocked(state, requirements),
  }
}

export function buildItemPanels(
  state: GameplayLikeState,
  itemsMap: Record<string, Item>,
): GameplayItemPanel[] {
  const classificationItems: Record<
    string,
    Array<{ id: string; name: string; quantity: number }>
  > = {}
  // Aggregate quantities across all states for display
  const totals: Record<string, number> = {}
  for (const entry of state.inventory) {
    if (entry.qty <= 0) continue
    totals[entry.id] = (totals[entry.id] || 0) + entry.qty
  }
  for (const [itemId, qty] of Object.entries(totals)) {
    const item = itemsMap[itemId]
    const classification = item?.classification || 'other'
    classificationItems[classification] = classificationItems[classification] || []
    classificationItems[classification].push({
      id: itemId,
      name: item?.name || itemId,
      quantity: qty,
    })
  }

  const panels: GameplayItemPanel[] = []
  const orderedClassifications = [
    ...CLASSIFICATION_ORDER,
    ...Object.keys(classificationItems)
      .filter((k) => !CLASSIFICATION_ORDER.includes(k))
      .sort(),
  ]
  for (const classification of orderedClassifications) {
    const itemsInClass = classificationItems[classification]
    if (!itemsInClass?.length) continue
    itemsInClass.sort((a, b) => a.name.localeCompare(b.name))
    panels.push({
      classification,
      title: CLASSIFICATION_NAME_MAP[classification] || classification,
      items: itemsInClass,
    })
  }
  return panels
}

function slotBase(slotId: string): string {
  return slotId.split('#', 1)[0]!
}

function slotLabel(slotType: 'tool' | 'equipment', slotId: string): string {
  const base = slotBase(slotId)
  if (slotType === 'tool') return TOOL_SLOT_NAME_MAP[base] || base
  return EQUIPMENT_SLOT_NAME_MAP[base] || base
}

function buildSlotInstances(
  slotType: 'tool' | 'equipment',
  attributes: Record<string, number>,
): string[] {
  const bases = slotType === 'tool' ? TOOL_SLOT_ORDER : EQUIPMENT_SLOT_ORDER
  const out: string[] = []
  for (const base of bases) {
    const rawCount = attributes[`${base}_slot_count`] ?? 1
    const count = Math.max(1, Math.floor(toFloat(rawCount, 1)))
    if (count <= 1) {
      out.push(base)
      continue
    }
    for (let idx = 1; idx <= count; idx++) {
      out.push(`${base}#${idx}`)
    }
  }
  return out
}

function requirementsFromItem(
  itemData: Record<string, unknown>,
  slotType: 'tool' | 'equipment',
): Record<string, number> {
  const reqMap: Record<string, number> = {}
  if (slotType === 'tool') {
    const reqs = ((itemData.tool_details as Record<string, unknown>)?.tool_position_requirement ||
      []) as Array<Record<string, unknown>>
    for (const req of reqs) {
      const base = String(req.tool_position || '')
      if (!base) continue
      const count = Math.max(1, Math.floor(toFloat(req.value, 1)))
      reqMap[base] = (reqMap[base] || 0) + count
    }
  } else {
    const reqs = ((itemData.equipment_details as Record<string, unknown>)
      ?.equipment_position_requirements || []) as Array<Record<string, unknown>>
    for (const req of reqs) {
      const base = String(req.position || '')
      if (!base || base === 'nothing') continue
      const count = Math.max(1, Math.floor(toFloat(req.value, 1)))
      reqMap[base] = (reqMap[base] || 0) + count
    }
  }
  return reqMap
}

export function buildEquipmentView(
  state: GameplayLikeState,
  itemsMap: Record<string, Item>,
): GameplayData['equipment_view'] {
  function buildSlotCells(
    slotType: 'tool' | 'equipment',
  ): GameplayData['equipment_view']['production_slots'] {
    const rows = slotType === 'tool' ? state.tools : state.equipment
    const slots = buildSlotInstances(slotType, state.attributes)
    const cells: GameplayData['equipment_view']['production_slots'] = []
    for (const slotId of slots) {
      const itemId = rows[slotId] || null
      let anchorSlot: string | null = null
      // For simplicity, anchor_slot === slotId when equipped (frontend doesn't have anchor granularity)
      if (itemId) anchorSlot = slotId
      cells.push({
        slot_type: slotType,
        slot_id: slotId,
        slot_name: slotLabel(slotType, slotId),
        item_id: itemId,
        item_name: itemId ? itemsMap[itemId]?.name || itemId : null,
        anchor_slot: anchorSlot,
        is_disabled: false,
        enhance_level: 0,
        enhance_fail_count: 0,
        attribute_preview: [],
      } as any)
    }
    return cells
  }

  const equipableItems: Array<{
    id: string
    name: string
    quantity: number
    slot_type: 'tool' | 'equipment'
    required_slots: string[]
  }> = []
  // Aggregate quantities across all states for display
  const totals: Record<string, number> = {}
  for (const entry of state.inventory) {
    if (entry.qty <= 0) continue
    totals[entry.id] = (totals[entry.id] || 0) + entry.qty
  }
  for (const [itemId, qty] of Object.entries(totals)) {
    const itemData = itemsMap[itemId]
    if (!itemData) continue

    if (itemData.tool) {
      const reqMap = requirementsFromItem(itemData as unknown as Record<string, unknown>, 'tool')
      equipableItems.push({
        id: itemId,
        name: itemData.name || itemId,
        quantity: qty,
        slot_type: 'tool',
        required_slots: Object.keys(reqMap).sort(),
      })
    }
    if (itemData.equipment) {
      const reqMap = requirementsFromItem(
        itemData as unknown as Record<string, unknown>,
        'equipment',
      )
      equipableItems.push({
        id: itemId,
        name: itemData.name || itemId,
        quantity: qty,
        slot_type: 'equipment',
        required_slots: Object.keys(reqMap).sort(),
      })
    }
  }

  return {
    production_slots: buildSlotCells('tool') as any,
    battle_slots: buildSlotCells('equipment') as any,
    equipable_items: equipableItems.sort((a, b) =>
      (a.slot_type + a.name).localeCompare(b.slot_type + b.name),
    ),
  }
}

function resolveQueueItem(raw: unknown): {
  event_id: string
  iterations?: number | null
  completed?: number
} {
  if (typeof raw === 'string') {
    return { event_id: raw, iterations: null, completed: 0 }
  }
  if (raw && typeof raw === 'object') {
    const obj = raw as Record<string, unknown>
    const iters = obj.iterations
    return {
      event_id: String(obj.event_id || ''),
      iterations: typeof iters === 'number' && iters > 0 ? iters : null,
      completed: Math.max(0, Math.floor(typeof obj.completed === 'number' ? obj.completed : 0)),
    }
  }
  return { event_id: '', iterations: null, completed: 0 }
}

export function buildQueue(
  state: GameplayLikeState,
  eventsMap: Record<string, Record<string, unknown>>,
): GameplayData['queue'] {
  const queueJson = state.queue_items || []
  const index = state.queue_index ?? 0
  const progress = state.queue_progress_seconds ?? 0

  const items: GameplayData['queue']['items'] = []
  for (let i = 0; i < queueJson.length; i++) {
    const item = resolveQueueItem(queueJson[i])
    if (!item.event_id) continue
    const event = eventsMap[item.event_id]
    if (!event) continue
    const remaining =
      item.iterations != null ? Math.max(0, item.iterations - (item.completed ?? 0)) : null
    items.push({
      index: i,
      event_id: item.event_id,
      name: String(event.name || item.event_id),
      type: String(event.type || 'unknown'),
      map: String(event.map || 'unknown'),
      map_name: MAP_NAME_MAP[String(event.map || 'unknown')] || String(event.map || 'unknown'),
      is_current: i === index,
      is_executable: checkRequirements(
        state,
        (event.requirements || []) as Array<Record<string, unknown>>,
      ),
      iterations: item.iterations,
      completed: item.completed,
      remaining,
    })
  }

  return { items, index, progress_seconds: progress }
}

export function buildActiveLoop(
  state: GameplayLikeState,
  eventsMap: Record<string, Record<string, unknown>>,
): GameplayData['active_loop'] {
  const queueJson = state.queue_items || []
  const index = state.queue_index ?? 0
  if (index < 0 || index >= queueJson.length) return null

  const item = resolveQueueItem(queueJson[index])
  const eventId = item.event_id
  if (!eventId) return null
  const event = eventsMap[eventId]
  if (!event || event.type !== 'loop') return null

  const duration = effectiveLoopTime(event, state.attributes)
  const elapsed = Math.max(0, toFloat(state.queue?.progress_seconds, 0))

  // estimateAffordableIterations simplified
  const reqs = (event.requirements || []) as Array<Record<string, unknown>>
  let affordable: number | null = null
  if (checkRequirements(state, reqs)) {
    const limits: number[] = []
    for (const req of reqs) {
      if (req.comparison_types !== undefined && req.comparison_types !== null) continue
      const rtype = String(req.type || '')
      if (rtype !== 'item' && rtype !== 'fluid') continue
      const cost = Math.max(0, Math.floor(toFloat(req.value, 0)))
      if (cost <= 0) continue
      const qty =
        rtype === 'item'
          ? itemQuantity(state, String(req.id || ''))
          : fluidQuantity(state, String(req.id || ''))
      limits.push(Math.floor(qty / cost))
    }
    if (limits.length) affordable = Math.max(0, Math.min(...limits))
  } else {
    affordable = 0
  }

  // Respect queue item iteration limit
  if (item.iterations != null && item.iterations > 0) {
    const remaining = Math.max(0, item.iterations - (item.completed ?? 0))
    affordable = affordable != null ? Math.min(affordable, remaining) : remaining
  }

  return {
    event_id: eventId,
    elapsed_seconds: elapsed,
    duration_seconds: duration,
    available_iterations: affordable,
  }
}

// ---------------------------------------------------------------------------
// Profile computation (mirroring backend)
// ---------------------------------------------------------------------------
const BATTLE_BASE_DATA = {
  hp: 100.0,
  mp: 100.0,
  sp: 100.0,
  physical_power: 20.0,
  magic_power: 20.0,
  attack_interval: 2.0,
  critical: 0.0,
  critical_rate: 2.0,
  block: 20.0,
  block_possibility_multiplier: 0.0,
  block_rate: 0.0,
  accuracy: 40.0,
  accuracy_possibility_multiplier: 0.0,
  evade: 20.0,
  evade_possibility_multiplier: 0.0,
  magic_instance: 0.33,
  final_damage_multiplier: 0.0,
  defense: 10.0,
  final_damage_reduce: 0.0,
  hatred: 100.0,
  hp_recovery: 0.0,
  mp_recovery: 0.0,
  sp_recovery: 0.0,
} as const

const BATTLE_ATTR_VIEW: Array<[string, string, boolean]> = [
  ['hp', '生命上限', false],
  ['mp', '魔力上限', false],
  ['sp', '耐力上限', false],
  ['physical_power', '物理攻击', false],
  ['magic_power', '奥术攻击', false],
  ['attack_interval', '攻击间隔(秒)', false],
  ['critical', '暴击率', true],
  ['critical_rate', '暴击倍率', false],
  ['block', '格挡值', false],
  ['block_possibility_multiplier', '格挡概率加成', true],
  ['block_rate', '格挡减伤', true],
  ['accuracy', '精准值', false],
  ['accuracy_possibility_multiplier', '命中概率加成', true],
  ['evade', '闪避值', false],
  ['evade_possibility_multiplier', '闪避概率加成', true],
  ['magic_instance', '奥术抵抗', true],
  ['defense', '防御值', false],
  ['hatred', '仇恨值', false],
  ['final_damage_multiplier', '最终伤害加成', true],
  ['final_damage_reduce', '最终伤害减免', true],
  ['hp_recovery', '生命恢复/秒', false],
  ['mp_recovery', '魔力恢复/秒', false],
  ['sp_recovery', '耐力恢复/秒', false],
]

export function getLevelProductionMultiplier(level: number, table: number[]): number {
  const lv = Math.max(1, Math.floor(level))
  if (lv <= table.length) {
    return table[lv - 1]!
  }
  if (table.length >= 2 && table[table.length - 2]! > 0) {
    const growthRatio = table[table.length - 1]! / table[table.length - 2]!
    return table[table.length - 1]! * Math.pow(growthRatio, lv - table.length)
  }
  return 1.0
}

function resolveItemAbilityMultiplier(itemData: Item, enhanceLevel: number): number {
  const details = (itemData.upgrade_details || {}) as Record<string, unknown>
  const curve = (details.upgrade_curve || []) as Array<Record<string, unknown>>
  const pointsMap = new Map<number, number>([[0, 1.0]])
  for (const row of curve) {
    const lv = Number(row.level)
    const mul = Number(row.ability_multiplier)
    if (!Number.isFinite(lv) || !Number.isFinite(mul)) continue
    pointsMap.set(Math.floor(lv), mul)
  }
  const points = Array.from(pointsMap.entries()).sort((a, b) => a[0] - b[0])
  if (!points.length) return 1.0

  const level = Math.max(0, Math.floor(enhanceLevel))
  if (level <= points[0]![0]) return points[0]![1]
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1]!
    const cur = points[i]!
    if (level > cur[0]) continue
    if (cur[0] === prev[0]) return cur[1]
    const ratio = (level - prev[0]) / (cur[0] - prev[0])
    return prev[1] + (cur[1] - prev[1]) * ratio
  }
  return points[points.length - 1]![1]
}

function buildScaledBasicData(
  basicRaw: Record<string, unknown>,
  upgradeRaw: Record<string, unknown>,
  abilityMultiplier: number,
): Record<string, number> {
  const out: Record<string, number> = {}
  const keys = new Set<string>([...Object.keys(basicRaw), ...Object.keys(upgradeRaw)])
  for (const key of keys) {
    const baseVal = toFloat(basicRaw[key], 0)
    const incVal = toFloat(upgradeRaw[key], 0)
    const value = baseVal + incVal * abilityMultiplier
    if (Math.abs(value) < 1e-12) continue
    out[key] = value
  }
  return out
}

function collectEquipmentPieces(
  state: GameplayLikeState,
  itemsMap: Record<string, Item>,
): Array<{ id: string; type: string; basic: Record<string, number> }> {
  const slotMeta = new Map<
    string,
    { anchor: string; item_id: string | null; enhance_level: number }
  >()
  for (const slot of state.equipment_view?.battle_slots || []) {
    slotMeta.set(slot.slot_id, {
      anchor: slot.anchor_slot || slot.slot_id,
      item_id: slot.item_id,
      enhance_level: Math.max(0, Math.floor(toFloat(slot.enhance_level, 0))),
    })
  }

  const out: Array<{ id: string; type: string; basic: Record<string, number> }> = []
  const seen = new Set<string>()
  for (const [slotId, itemId] of Object.entries(state.equipment)) {
    if (!itemId) continue
    const meta = slotMeta.get(slotId)
    const anchor = meta?.anchor || slotId
    const enhanceLevel = meta?.enhance_level ?? 0
    const key = `${anchor}:${itemId}`
    if (seen.has(key)) continue
    seen.add(key)
    const itemData = itemsMap[itemId]
    if (!itemData || !itemData.equipment) continue
    const details = (itemData.equipment_details || {}) as Record<string, unknown>
    const basicRaw = (details.equipment_basic_data || {}) as Record<string, unknown>
    const upgradeRaw = (details.equipment_upgrade_data || {}) as Record<string, unknown>
    const ability = resolveItemAbilityMultiplier(itemData, enhanceLevel)
    const basic = buildScaledBasicData(basicRaw, upgradeRaw, ability)
    out.push({
      id: itemId,
      type: String(details.type || ''),
      basic,
    })
  }
  return out
}

export function buildProfile(
  state: GameplayLikeState,
  itemsMap: Record<string, Item>,
  levelProductionTable: number[],
): {
  production_attributes: GameplayProfileProductionAttribute[]
  battle_attributes: GameplayProfileBattleAttribute[]
} {
  // Production attributes
  const productionAttributes: GameplayProfileProductionAttribute[] = []
  for (const skillId of PRODUCTION_SKILL_ORDER) {
    const skillObj = state.skills[skillId]
    const baseLevel = skillObj ? Math.max(0, Math.floor(toFloat(skillObj.level, 0))) : 1
    const levelBuff = toFloat(state.attributes[`${skillId}_level_buff`], 0)
    const effectiveLevel = Math.max(1, Math.floor(baseLevel + levelBuff))
    const productionBonus = toFloat(state.attributes[`${skillId}_production_multiplier`], 0)
    const speedBonus = toFloat(state.attributes[`${skillId}_speed_multiplier`], 0)
    const levelMultiplier = getLevelProductionMultiplier(effectiveLevel, levelProductionTable)
    const totalOutputMultiplier = Math.max(0, (1 + productionBonus) * levelMultiplier)
    const totalSpeedMultiplier = Math.max(0.05, 1 + speedBonus)
    productionAttributes.push({
      skill_id: skillId,
      skill_name: SKILL_NAME_MAP[skillId] || skillId,
      base_level: baseLevel,
      effective_level: effectiveLevel,
      level_multiplier: levelMultiplier,
      production_multiplier: productionBonus,
      speed_multiplier: speedBonus,
      total_output_multiplier: totalOutputMultiplier,
      total_speed_multiplier: totalSpeedMultiplier,
    })
  }

  // Battle attributes
  const pieces = collectEquipmentPieces(state, itemsMap)

  function sumAttr(...keys: string[]): number {
    let total = 0
    for (const piece of pieces) {
      for (const key of keys) {
        if (key in piece.basic) {
          total += toFloat(piece.basic[key], 0)
        }
      }
    }
    return total
  }

  function productAttr(...keys: string[]): number {
    let result = 1.0
    let hasValue = false
    for (const piece of pieces) {
      for (const key of keys) {
        if (key in piece.basic) {
          result *= 1.0 + toFloat(piece.basic[key], 0)
          hasValue = true
        }
      }
    }
    return hasValue ? result : 1.0
  }

  const resilienceLv = skillLevel(state, 'resilience')
  const staminaLv = skillLevel(state, 'stamina')
  const intelligenceLv = skillLevel(state, 'intelligence')
  const strengthLv = skillLevel(state, 'strength')
  const rangingLv = skillLevel(state, 'ranging')
  const defenseLv = skillLevel(state, 'defense')
  const magicLv = skillLevel(state, 'magic')

  const resilienceBonusLv = Math.max(0, resilienceLv - 1)
  const staminaBonusLv = Math.max(0, staminaLv - 1)
  const intelligenceBonusLv = Math.max(0, intelligenceLv - 1)
  const strengthBonusLv = Math.max(0, strengthLv - 1)
  const rangingBonusLv = Math.max(0, rangingLv - 1)
  const defenseBonusLv = Math.max(0, defenseLv - 1)
  const magicBonusLv = Math.max(0, magicLv - 1)

  const hp =
    (BATTLE_BASE_DATA['hp'] + 5.0 * resilienceBonusLv + sumAttr('max_hp', 'hp')) *
    productAttr('max_hp_multiplier', 'hp_multiplier')
  const sp =
    (BATTLE_BASE_DATA['sp'] + 1.0 * staminaBonusLv + sumAttr('max_sp', 'sp')) *
    productAttr('max_sp_multiplier', 'sp_multiplier')
  const mp =
    (BATTLE_BASE_DATA['mp'] + 1.0 * intelligenceBonusLv + sumAttr('max_mp', 'mp')) *
    productAttr('max_mp_multiplier', 'mp_multiplier')

  const powerMultiplier = productAttr('power_multiplier')
  const physicalPower =
    (BATTLE_BASE_DATA['physical_power'] + 1.0 * strengthBonusLv + sumAttr('physical_power')) *
    powerMultiplier *
    Math.pow(1.005, strengthBonusLv)
  const magicPower =
    (BATTLE_BASE_DATA['magic_power'] +
      1.0 * magicBonusLv +
      sumAttr('magic_power', 'magic_damage')) *
    powerMultiplier *
    Math.pow(1.005, magicBonusLv)

  const weaponIntervals: number[] = []
  for (const piece of pieces) {
    if (piece.type !== 'weapon') continue
    const interval = toFloat(piece.basic['attack_interval'], 0)
    if (interval > 0) weaponIntervals.push(interval)
  }
  const weaponInterval = weaponIntervals.length
    ? Math.max(...weaponIntervals)
    : BATTLE_BASE_DATA['attack_interval']
  const attackInterval = Math.max(
    0.1,
    weaponInterval / Math.max(0.05, productAttr('attack_speed', 'final_attack_speed_multiplier')),
  )

  const critical = Math.min(
    1.0,
    Math.max(
      0.0,
      (BATTLE_BASE_DATA['critical'] + sumAttr('critical')) *
        productAttr('critical_possibility_multiplier'),
    ),
  )
  const criticalRate =
    BATTLE_BASE_DATA['critical_rate'] + sumAttr('critical_rate', 'critical_multiplier')

  const block =
    (BATTLE_BASE_DATA['block'] + 1.0 * defenseBonusLv + sumAttr('block')) *
    productAttr('block_multiplier')
  const blockPossibilityMultiplier =
    (1.0 + BATTLE_BASE_DATA['block_possibility_multiplier']) *
      productAttr('block_possibility_multiplier') -
    1.0
  const blockRate =
    (BATTLE_BASE_DATA['block_rate'] + sumAttr('block_rate')) * productAttr('block_rate_multiplier')

  const recoveryMultiplier = productAttr('overall_recovery_speed')
  const hpRecovery = (BATTLE_BASE_DATA['hp_recovery'] + sumAttr('hp_recovery')) * recoveryMultiplier
  const spRecovery =
    (BATTLE_BASE_DATA['sp_recovery'] + 0.02 * staminaBonusLv + sumAttr('sp_recovery')) *
    recoveryMultiplier
  const mpRecovery =
    (BATTLE_BASE_DATA['mp_recovery'] + 0.02 * intelligenceBonusLv + sumAttr('mp_recovery')) *
    recoveryMultiplier

  const accuracy =
    (BATTLE_BASE_DATA['accuracy'] + 0.5 * rangingBonusLv + sumAttr('accuracy')) *
    productAttr('accuracy_multiplier')
  const accuracyPossibilityMultiplier =
    (1.0 + BATTLE_BASE_DATA['accuracy_possibility_multiplier']) *
      productAttr('accuracy_possibility_multiplier') -
    1.0

  const evade =
    (BATTLE_BASE_DATA['evade'] + 0.5 * rangingBonusLv + sumAttr('evade')) *
    productAttr('evade_multiplier')
  const evadePossibilityMultiplier =
    (1.0 + BATTLE_BASE_DATA['evade_possibility_multiplier']) *
      productAttr('evade_possibility_multiplier') -
    1.0

  const magicInstance =
    (BATTLE_BASE_DATA['magic_instance'] + sumAttr('magic_instance')) *
    productAttr('magic_instance_multiplier')
  const defense =
    (BATTLE_BASE_DATA['defense'] + 1.0 * defenseBonusLv + sumAttr('defense')) *
    productAttr('defense_multiplier')
  const hatred = (BATTLE_BASE_DATA['hatred'] + sumAttr('hatred')) * productAttr('hatred_multiplier')

  const finalDamageMultiplier =
    (1.0 + BATTLE_BASE_DATA['final_damage_multiplier']) * productAttr('final_damage_multiplier') -
    1.0
  const finalDamageReduce =
    (1.0 + BATTLE_BASE_DATA['final_damage_reduce']) *
      productAttr('final_damage_reduce', 'final_damage_induce') -
    1.0

  const computedBattle: Record<string, number> = {
    hp,
    mp,
    sp,
    physical_power: physicalPower,
    magic_power: magicPower,
    attack_interval: attackInterval,
    critical,
    critical_rate: criticalRate,
    block,
    block_possibility_multiplier: blockPossibilityMultiplier,
    block_rate: blockRate,
    accuracy,
    accuracy_possibility_multiplier: accuracyPossibilityMultiplier,
    evade,
    evade_possibility_multiplier: evadePossibilityMultiplier,
    magic_instance: magicInstance,
    defense,
    hatred,
    final_damage_multiplier: finalDamageMultiplier,
    final_damage_reduce: finalDamageReduce,
    hp_recovery: hpRecovery,
    mp_recovery: mpRecovery,
    sp_recovery: spRecovery,
  }

  const battleAttributes: GameplayProfileBattleAttribute[] = []
  const battleBaseDict = BATTLE_BASE_DATA as unknown as Record<string, number>
  for (const [attrId, attrName, asPercent] of BATTLE_ATTR_VIEW) {
    battleAttributes.push({
      id: attrId,
      name: attrName,
      base: battleBaseDict[attrId] ?? 0,
      value: toFloat(computedBattle[attrId] ?? battleBaseDict[attrId] ?? 0, 0),
      as_percent: asPercent,
    })
  }

  return { production_attributes: productionAttributes, battle_attributes: battleAttributes }
}
