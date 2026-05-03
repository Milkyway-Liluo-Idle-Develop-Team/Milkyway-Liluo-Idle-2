export interface BattleListItem {
  id: string
  name: string
  map: string
  interval: number
}

export interface BattleState {
  battle_id: string
  battle_name: string
  map: string
  status: 'fighting' | 'between_waves' | 'respawn' | 'stopped' | string
  time: number
  wave_number: number
  wave_type: string | null
  next_step_in_seconds: number | null
  player: {
    name: string
    alive: boolean
    hp: number
    max_hp: number
    mp: number
    max_mp: number
    sp: number
    max_sp: number
    next_ready_in_seconds: number
    action_cooldown_seconds: number
    action_cooldown_progress: number
    last_skill_id: string
    last_skill_name: string
  }
  enemies: Array<{
    instance_id: string
    enemy_id: string
    name: string
    alive: boolean
    hp: number
    max_hp: number
    next_ready_in_seconds: number
    action_cooldown_seconds: number
    action_cooldown_progress: number
    last_skill_id: string
    last_skill_name: string
  }>
  logs: Array<Record<string, unknown>>
}
