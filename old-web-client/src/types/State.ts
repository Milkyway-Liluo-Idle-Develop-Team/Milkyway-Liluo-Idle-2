// ---------------------------------------------------------------------------
// Raw player state — exactly what the server owns and mutates.
// Delta patches only touch these fields.
// ---------------------------------------------------------------------------

export interface InventoryEntry {
  id: string
  state?: number
  qty: number
}

export interface RawState {
  skills: Record<string, { level: number; exp: number }>
  inventory: InventoryEntry[]
  equipment: Record<string, string | null>
  tools: Record<string, string | null>
  event_counts: Record<string, number>
  seen_items: string[]
  unlocked_events: string[]
  attributes: Record<string, number>
  queue_items: Array<unknown>
  queue_index: number
  queue_progress_seconds: number
}

export type RawStatePatch = Partial<RawState> & Record<string, unknown>
