import { send } from './ws'
import { USE_JSON } from './config'
import {
  QueueAppendReq,
  QueueRemoveReq,
  QueueMoveReq,
  type QueueSetEntry,
  QueueSetReq,
} from '@/pb/queue'
import { EquipReq, UnequipReq } from '@/pb/equipment'
import type { BattleListItem, BattleState } from '@/types/BattleResponse'

// ---------------------------------------------------------------------------
// ID Registry — populated by the store after loading static config
// ---------------------------------------------------------------------------
let idRegistry: {
  items: Record<string, number>
  events: Record<string, number>
  skills: Record<string, number>
} | null = null

export function setActionIdRegistry(registry: {
  items: Record<string, number>
  events: Record<string, number>
  skills: Record<string, number>
}) {
  idRegistry = registry
}

function itemNumId(strId: string): number {
  if (!idRegistry) throw new Error('ID registry not loaded')
  const num = idRegistry.items[strId]
  if (num === undefined) throw new Error(`Unknown item ID: ${strId}`)
  return num
}

function eventNumId(strId: string): number {
  if (!idRegistry) throw new Error('ID registry not loaded')
  const num = idRegistry.events[strId]
  if (num === undefined) throw new Error(`Unknown event ID: ${strId}`)
  return num
}

// ---------------------------------------------------------------------------
// State sync (new backend: state comes via WS push, no explicit sync needed)
// ---------------------------------------------------------------------------
export async function syncAndSettle(): Promise<Record<string, unknown> | undefined> {
  return undefined
}

export async function requestSync(): Promise<void> {
  // No-op in new backend
}

// ---------------------------------------------------------------------------
// Loop / Instant / Upgrade — all map to queue operations in new backend
// ---------------------------------------------------------------------------
function queueAppendPayload(eventId: string, iterations?: number) {
  const data = {
    eventId: eventNumId(eventId),
    targetCycles: iterations ?? -1,
    queueId: 0,
  }
  return USE_JSON ? data : QueueAppendReq.encode(data).finish()
}

export async function startLoop(eventId: string, iterations?: number): Promise<void> {
  await send('queue.append', queueAppendPayload(eventId, iterations))
}

export async function stopLoop(): Promise<void> {
  // New backend auto-executes queue; no explicit stop.
}

export async function queueAppend(eventId: string, iterations?: number): Promise<void> {
  await send('queue.append', queueAppendPayload(eventId, iterations))
}

export async function queueRemove(index: number): Promise<void> {
  const data = { position: index, queueId: 0 }
  await send('queue.remove', USE_JSON ? data : QueueRemoveReq.encode(data).finish())
}

export async function queueSwap(fromIndex: number, toIndex: number): Promise<void> {
  const data = { fromPosition: fromIndex, toPosition: toIndex, queueId: 0 }
  await send('queue.move', USE_JSON ? data : QueueMoveReq.encode(data).finish())
}

export async function queueBringToFront(index: number): Promise<void> {
  const data = { fromPosition: index, toPosition: 0, queueId: 0 }
  await send('queue.move', USE_JSON ? data : QueueMoveReq.encode(data).finish())
}

export async function executeInstant(eventId: string): Promise<void> {
  await send('queue.append', queueAppendPayload(eventId, 1))
}

export async function executeUpgrade(eventId: string): Promise<void> {
  await send('queue.append', queueAppendPayload(eventId, 1))
}

export async function queueSet(entries: Array<{ eventId: string; targetCycles: number }>): Promise<void> {
  const pbEntries: QueueSetEntry[] = entries.map((e) => ({
    eventId: eventNumId(e.eventId),
    targetCycles: e.targetCycles,
  }))
  const data = { entries: pbEntries, queueId: 0 }
  await send('queue.set', USE_JSON ? data : QueueSetReq.encode(data).finish())
}

// ---------------------------------------------------------------------------
// Equipment
// ---------------------------------------------------------------------------
export async function equipItem(itemId: string, slot: string): Promise<void> {
  const data = { itemId: itemNumId(itemId), itemState: 0, slot }
  await send('inventory.equip', USE_JSON ? data : EquipReq.encode(data).finish())
}

export async function unequipItem(slot: string): Promise<void> {
  const data = { slot }
  await send('inventory.unequip', USE_JSON ? data : UnequipReq.encode(data).finish())
}

// ---------------------------------------------------------------------------
// Battle (backend placeholder — return empty to keep UI from crashing)
// ---------------------------------------------------------------------------
export async function fetchBattleList(): Promise<BattleListItem[]> {
  return []
}

export async function syncBattleState(): Promise<BattleState | null> {
  return null
}

export async function startBattle(battleId: string): Promise<BattleState> {
  throw new Error('Battle system not yet implemented in new backend')
}

export async function stopBattle(): Promise<BattleState> {
  throw new Error('Battle system not yet implemented in new backend')
}

// ---------------------------------------------------------------------------
// Enhance (backend not yet implemented)
// ---------------------------------------------------------------------------
export async function enhancePreview(slotType: string, anchorSlot: string): Promise<unknown> {
  throw new Error('Enhance system not yet implemented in new backend')
}

export async function enhanceExecute(slotType: string, anchorSlot: string): Promise<unknown> {
  throw new Error('Enhance system not yet implemented in new backend')
}

// ---------------------------------------------------------------------------
// Debug
// ---------------------------------------------------------------------------
export async function skipTime(seconds: number): Promise<{ log: Array<{ event_id: string; iterations: number; experience: number }> }> {
  throw new Error('skip_time not available in new backend')
}
