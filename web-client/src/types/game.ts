export interface GameConfig {
  actions: {
    items: ItemDef[]
    events: EventDef[]
    enemies: EnemyDef[]
    battles: BattleDef[]
  }
  id_registry: {
    items: Record<string, number>
    events: Record<string, number>
    skills: Record<string, number>
    maps: Record<string, number>
    battle_skills: Record<string, number>
  }
  attributes: Record<string, AttributeDef>
  attr_registry: Record<string, number>
  level_curve_csv: string
}

export interface ItemDef {
  id: string
  name: string
  classification: string
  tool: boolean
  equipment: boolean
  upgradable: boolean
  tool_details?: ToolDetails
  equipment_details?: EquipmentDetails
  upgrade_details?: UpgradeDetails
}

export interface ToolDetails {
  tool_position_requirement: Array<{ tool_position: string; value: number }>
  tool_basic_data: Record<string, number>
  tool_type: string
  tool_upgrade_data: Record<string, number>
}

export interface EquipmentDetails {
  type: string
  equipment_position_requirements: Array<{ position: string; value: number }>
  element: string
  battle_skills: BattleSkillDef[]
  equipment_basic_data: Record<string, number>
  equipment_upgrade_data: Record<string, number>
}

export interface BattleSkillDef {
  id: string
  name: string
  description: string
  target_type: string
  damage: { type: string; flat: number; multiplier: number }
  is_basic: boolean
}

export interface UpgradeDetails {
  max_upgrade: number
  enhance_slot: number
  forge_slot: number
  upgrade_curve: Array<{ level: number; recommend_level: number; basic_success_rate: number; ability_multiplier: number }>
}

export interface EventDef {
  id: string
  type: 'loop' | 'instant' | 'upgrade'
  name: string
  description: string
  need_skill: string
  map: string
  requirements: EventRequirement[]
  rewards: EventReward[]
  loop_time?: number
  repeat_time?: number
}

export interface EventRequirement {
  type: string
  id: string
  comparison_types?: string
  value?: number
}

export interface EventReward {
  type?: string
  id?: string
  num?: number
  value?: number
  skill_id?: string
}

export interface EnemyDef {
  id: string
  name: string
  enemy_battle_data: Record<string, number>
  basic_damage_type: string
  battle_skill: EnemyBattleSkill[]
  rewards: EventReward[]
}

export interface EnemyBattleSkill {
  id: string
  priority: number
  condition: string
  skill_id: string
}

export interface BattleDef {
  id: string
  name: string
  map: string
  weak_enemy_combinations: Array<{ enemies: string[]; weight: number }>
  strong_enemy_combinations: Array<{ enemies: string[]; weight: number }>
  boss_enemy_combinations: Array<{ enemies: string[]; weight: number }>
  combination_loop: string[]
}

export interface AttributeDef {
  name: string
  default_value: number
  min_value: number
  direction: string
  group: string
}

// --- Runtime state ---

export interface InventoryItem {
  item_id: number
  item_state: number
  quantity: number
}

export interface SkillState {
  skill_id: number
  level: number
  xp: number
}

export interface AttributeState {
  attr_id: string
  name: string
  final_value: number
  group: string
  direction: string
  modifiers: ModifierWire[]
}

export interface ModifierWire {
  source: string
  op: string
  value: number
  ref_attr?: string
  display?: string
}

export interface BestiaryEntry {
  type: string
  id: string
}

export interface EventExecution {
  queue_id: number
  event_id: number
  target_cycles: number
  progress: number
}

export interface EventQueueEntry {
  position: number
  event_id: number
  target_cycles: number
  progress: number
}

export interface EquipmentSlot {
  slot: string
  item_id: number
  item_state: number
}

// --- StateFull / StateDiff ---

export interface StateFull {
  inventory: InventoryItem[]
  attribute: AttributeState[]
  skill_xp: SkillState[]
  bestiary: BestiaryEntry[]
  event_execution: EventExecution[]
  equipment: Record<string, { item_id: number; item_state: number }>
}

export interface InventoryDiff {
  item_id: number
  item_state: number
  quantity_delta: number
  reason: string
}

export interface AttributeDiff {
  attr_id: string
  final_value: number
  modifiers: ModifierWire[]
}

export interface SkillXPDiff {
  skill_id: number
  xp_delta: number
  new_level: number
}

export interface BestiaryDiff {
  type: string
  id: string
}

export interface EventExecutionDiff {
  event_id: number
  cycles: number
}

export interface EventQueueDiff {
  queue_id: number
  scope: string
  entries: EventQueueEntry[]
}

export interface EquipmentDiff {
  slot: string
  item_id: number
  item_state: number
  action: string
}

export interface StateDiff {
  inventory?: InventoryDiff[]
  attribute?: AttributeDiff[]
  skill_xp?: SkillXPDiff[]
  bestiary?: BestiaryDiff[]
  event_execution?: EventExecutionDiff[]
  event_queue?: EventQueueDiff[]
  equipment?: EquipmentDiff[]
}

// --- Display views ---

export interface GameplaySkill {
  id: string
  name: string
  level: number
  xp: number
  level_progress: number
  current_level_total_exp: number
  next_level_total_exp: number
  level_production_multiplier: number
}

export interface GameplayEvent {
  id: string
  name: string
  description: string
  type: 'loop' | 'upgrade' | string
  map: string
  map_name: string
  need_skill: string
  need_skill_name: string
  loop_time?: number
  effective_loop_time?: number | null
  experience?: number
  event_count: number
  max_executions?: number | null
  required_skills: Array<{
    skill_id: string
    skill_name: string
    comparison_types: string
    comparison_text: string
    value: number
  }>
  cost_items: Array<{
    item_id: string
    item_name: string
    value: number
  }>
  reward_preview: Array<{
    item_id: string
    item_name: string
    base_value: number
    effective_value: number
  }>
  is_executable: boolean
  is_skill_blocked: boolean
}

export interface GameplayItemPanel {
  classification: string
  title: string
  items: Array<{
    id: string
    name: string
    quantity: number
  }>
}

export interface QueueItem {
  index: number
  event_id: string
  name: string
  type: string
  map: string
  map_name: string
  is_current: boolean
  is_executable: boolean
  iterations?: number | null
  completed?: number
  remaining?: number | null
}

export interface GameplayData {
  production_skills: GameplaySkill[]
  combat_skills: GameplaySkill[]
  profile?: {
    production_attributes: Array<{
      skill_id: string
      skill_name: string
      base_level: number
      effective_level: number
      level_multiplier: number
      production_multiplier: number
      speed_multiplier: number
      total_output_multiplier: number
      total_speed_multiplier: number
    }>
    battle_attributes: Array<{
      id: string
      name: string
      base: number
      value: number
      as_percent: boolean
    }>
  }
  maps: Array<{ id: string; name: string }>
  loop_events: GameplayEvent[]
  upgrade_events: GameplayEvent[]
  item_panels: GameplayItemPanel[]
  active_loop: {
    event_id: string
    elapsed_seconds: number
    duration_seconds: number
    available_iterations: number | null
  } | null
  queue: {
    items: QueueItem[]
    index: number
    progress_seconds: number
  }
}
