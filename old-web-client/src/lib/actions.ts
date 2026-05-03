import { send, sendAndWait, WsMessageType } from './ws'
import { postJson } from './api'
import type { BattleListItem, BattleState } from '@/types/BattleResponse'

export async function syncAndSettle(): Promise<Record<string, unknown> | undefined> {
  const msg = await sendAndWait(WsMessageType.SYNC, {}, WsMessageType.DELTA)
  const payload = msg.data as { patch?: Record<string, unknown> } | undefined
  return payload?.patch
}

export async function requestSync(): Promise<void> {
  await send(WsMessageType.SYNC, {})
}

export async function startLoop(eventId: string, iterations?: number): Promise<void> {
  await send(WsMessageType.ACTION_LOOP, { event_id: eventId, iterations })
}

export async function stopLoop(): Promise<void> {
  await send(WsMessageType.ACTION_LOOP_STOP, {})
}

export async function queueAppend(eventId: string, iterations?: number): Promise<void> {
  await send(WsMessageType.QUEUE_APPEND, { event_id: eventId, iterations })
}

export async function queueRemove(index: number): Promise<void> {
  await send(WsMessageType.QUEUE_REMOVE, { index })
}

export async function queueSwap(fromIndex: number, toIndex: number): Promise<void> {
  await send(WsMessageType.QUEUE_SWAP, { from_index: fromIndex, to_index: toIndex })
}

export async function queueBringToFront(index: number): Promise<void> {
  await send(WsMessageType.QUEUE_BRING_TO_FRONT, { index })
}

export async function executeInstant(eventId: string): Promise<void> {
  await send(WsMessageType.INSTANT, { event_id: eventId })
}

export async function executeUpgrade(eventId: string): Promise<void> {
  await send(WsMessageType.UPGRADE, { event_id: eventId })
}

export async function equipItem(itemId: string, slot: string): Promise<void> {
  await send(WsMessageType.EQUIP, { item_id: itemId, slot })
}

export async function unequipItem(slot: string): Promise<void> {
  await send(WsMessageType.UNEQUIP, { slot })
}

export async function fetchBattleList(): Promise<BattleListItem[]> {
  const msg = await sendAndWait(WsMessageType.BATTLE_LIST, {}, WsMessageType.BATTLE_LIST)
  return (msg.data as BattleListItem[]) ?? []
}

export async function syncBattleState(): Promise<BattleState | null> {
  const msg = await sendAndWait(WsMessageType.BATTLE_STATE, {}, WsMessageType.BATTLE_STATE)
  return (msg.data as BattleState | null) ?? null
}

export async function startBattle(battleId: string): Promise<BattleState> {
  const msg = await sendAndWait(
    WsMessageType.BATTLE_START,
    { battle_id: battleId, player_skills: [] },
    WsMessageType.BATTLE_STATE,
  )
  return msg.data as BattleState
}

export async function stopBattle(): Promise<BattleState> {
  const msg = await sendAndWait(WsMessageType.BATTLE_STOP, {}, WsMessageType.BATTLE_STATE)
  return msg.data as BattleState
}

export async function enhancePreview(slotType: string, anchorSlot: string): Promise<unknown> {
  const msg = await sendAndWait(
    WsMessageType.ENHANCE_PREVIEW,
    { slot_type: slotType, anchor_slot: anchorSlot },
    WsMessageType.ENHANCE_PREVIEW,
  )
  return msg.data
}

export async function enhanceExecute(slotType: string, anchorSlot: string): Promise<unknown> {
  const msg = await sendAndWait(
    WsMessageType.ENHANCE_EXECUTE,
    { slot_type: slotType, anchor_slot: anchorSlot },
    WsMessageType.ENHANCE_EXECUTE,
  )
  return msg.data
}

export async function skipTime(seconds: number): Promise<{ log: Array<{ event_id: string; iterations: number; experience: number }> }> {
  const res = await postJson<{ log: Array<{ event_id: string; iterations: number; experience: number }> }>('/api/debug/skip_time', { seconds }, { credentials: 'include' })
  if (!res.ok) {
    throw new Error(res.error)
  }
  return res.data
}
