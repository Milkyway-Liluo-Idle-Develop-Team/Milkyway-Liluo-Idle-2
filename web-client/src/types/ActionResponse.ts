// 根据后端 JSON 和你的 impl.ts 简化
export interface Item {
  id: string
  name: string
  tool: boolean
  equipment: boolean
  upgradable: boolean
  classification: string
  tool_details?: any
  equipment_details?: any
  upgrade_details?: any
}

export interface Event {
  id: string
  name: string
  description: string
  type: string
  need_skill: string
  requirements?: any[]
  loop_time?: number
  experience?: number
  rewards?: Array<{ id: string; num?: number; value?: number }>
  map?: string
}

export interface ActionsResponse {
  items: Item[]
  events: Event[]
  level_production: number[]
}