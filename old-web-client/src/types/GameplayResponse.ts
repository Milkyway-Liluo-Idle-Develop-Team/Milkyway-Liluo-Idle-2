import type { RawState } from './State'

export interface GameplaySkill {
  id: string
  name: string
  level: number
  exp: number
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

export interface GameplayProfileProductionAttribute {
  skill_id: string
  skill_name: string
  base_level: number
  effective_level: number
  level_multiplier: number
  production_multiplier: number
  speed_multiplier: number
  total_output_multiplier: number
  total_speed_multiplier: number
}

export interface GameplayProfileBattleAttribute {
  id: string
  name: string
  base: number
  value: number
  as_percent: boolean
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

// ---------------------------------------------------------------------------
// GameplayData = raw server state + computed display views.
// Delta patches only touch RawState fields; views are computed locally.
// ---------------------------------------------------------------------------
export interface GameplayData extends RawState {
  production_skills: GameplaySkill[]
  combat_skills: GameplaySkill[]
  profile?: {
    production_attributes: GameplayProfileProductionAttribute[]
    battle_attributes: GameplayProfileBattleAttribute[]
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
  equipment_view: {
    production_slots: Array<{
      slot_type: 'tool'
      slot_id: string
      slot_name: string
      item_id: string | null
      item_name: string | null
      anchor_slot: string | null
      is_disabled: boolean
      enhance_level: number | null
      enhance_fail_count: number | null
      attribute_preview: Array<{
        key: string
        value: number
      }>
    }>
    battle_slots: Array<{
      slot_type: 'equipment'
      slot_id: string
      slot_name: string
      item_id: string | null
      item_name: string | null
      anchor_slot: string | null
      is_disabled: boolean
      enhance_level: number | null
      enhance_fail_count: number | null
      attribute_preview: Array<{
        key: string
        value: number
      }>
    }>
    equipable_items: Array<{
      id: string
      name: string
      quantity: number
      slot_type: 'tool' | 'equipment'
      required_slots: string[]
    }>
  }
}
