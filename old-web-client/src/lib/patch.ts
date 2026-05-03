import type { InventoryEntry, RawState, RawStatePatch } from '@/types/State'

/**
 * Merge a server delta patch into local raw state.
 *
 * Rules:
 * - Dict fields are shallow-merged (Object.assign)
 * - Array fields are replaced entirely (state owns the reference)
 * - null/undefined/0/empty string in inventory/equipment/tools means "delete"
 */

function mergeDict(
  target: Record<string, unknown>,
  patch: Record<string, unknown>,
  deletable = false,
) {
  for (const [key, value] of Object.entries(patch)) {
    if (deletable && (value === null || value === undefined)) {
      delete target[key]
    } else {
      target[key] = value
    }
  }
}

function mergeNullableDict(
  target: Record<string, string | null>,
  patch: Record<string, string | null>,
) {
  for (const [key, value] of Object.entries(patch)) {
    if (value === null || value === undefined) {
      delete target[key]
    } else {
      target[key] = value
    }
  }
}

function mergeInventory(target: InventoryEntry[], patch: InventoryEntry[]) {
  for (const entry of patch) {
    const state = entry.state ?? 0
    const idx = target.findIndex((e) => e.id === entry.id && (e.state ?? 0) === state)
    if (entry.qty === null || entry.qty === undefined || entry.qty <= 0) {
      if (idx >= 0) target.splice(idx, 1)
    } else {
      if (idx >= 0) {
        target[idx] = { ...entry }
      } else {
        target.push({ ...entry })
      }
    }
  }
}

export function applyPatch(state: RawState, patch: RawStatePatch): void {
  if (!patch) return

  if (patch.skills) {
    mergeDict(state.skills as Record<string, unknown>, patch.skills as Record<string, unknown>)
  }
  if (patch.inventory) {
    mergeInventory(state.inventory, patch.inventory as InventoryEntry[])
  }
  if (patch.equipment) {
    mergeNullableDict(state.equipment, patch.equipment as Record<string, string | null>)
  }
  if (patch.tools) {
    mergeNullableDict(state.tools, patch.tools as Record<string, string | null>)
  }
  if (patch.event_counts) {
    mergeDict(
      state.event_counts as Record<string, unknown>,
      patch.event_counts as Record<string, unknown>,
    )
  }
  if (patch.new_seen_items) {
    const existing = new Set(state.seen_items)
    for (const item of patch.new_seen_items as string[]) {
      existing.add(item)
    }
    state.seen_items = Array.from(existing).sort()
  }
  if (patch.seen_items) {
    state.seen_items = patch.seen_items as string[]
  }
  if (patch.unlocked_events) {
    state.unlocked_events = patch.unlocked_events as string[]
  }
  if (patch.attributes) {
    mergeDict(state.attributes as Record<string, unknown>, patch.attributes as Record<string, unknown>)
  }
  if (patch.queue_items) {
    state.queue_items = patch.queue_items as Array<unknown>
  }
  if (patch.queue_index !== undefined) {
    state.queue_index = patch.queue_index as number
  }
  if (patch.queue_progress_seconds !== undefined) {
    state.queue_progress_seconds = patch.queue_progress_seconds as number
  }
}
