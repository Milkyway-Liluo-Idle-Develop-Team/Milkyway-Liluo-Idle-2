import { API_BASE_URL } from './api'

function getWsUrl(): string {
  const base = API_BASE_URL
  return base.replace(/^http/, 'ws') + '/ws'
}

// ============================================================================
// Message type enum — mirrors proto WsMessageType
// Switching to protobuf later only requires changing encode/decode below.
// ============================================================================
export enum WsMessageType {
  UNKNOWN = 'unknown',
  ERROR = 'error',
  DELTA = 'delta',
  GAMEPLAY = 'gameplay',
  GAMEPLAY_LIGHT = 'gameplay_light',

  ACTION_LOOP = 'action_loop',
  ACTION_LOOP_STOP = 'action_loop_stop',
  SYNC = 'sync',
  INSTANT = 'instant',
  UPGRADE = 'upgrade',
  EQUIP = 'equip',
  UNEQUIP = 'unequip',
  ENHANCE_PREVIEW = 'enhance_preview',
  ENHANCE_EXECUTE = 'enhance_execute',
  SET_QUEUE = 'set_queue',
  QUEUE_APPEND = 'queue_append',
  QUEUE_REMOVE = 'queue_remove',
  QUEUE_INSERT = 'queue_insert',
  QUEUE_REPLACE = 'queue_replace',
  QUEUE_SWAP = 'queue_swap',
  QUEUE_BRING_TO_FRONT = 'queue_bring_to_front',

  BATTLE_LIST = 'battle_list',
  BATTLE_START = 'battle_start',
  BATTLE_STATE = 'battle_state',
  BATTLE_STOP = 'battle_stop',
}

// ============================================================================
// Serialization abstraction
// Today: JSON. Tomorrow: protobuf WsMessage.encode / decode.
// ============================================================================
export type WsPayload = Record<string, unknown>

export interface WsMessage {
  type: WsMessageType
  req_id?: number
  [key: string]: unknown
}

function encodeMessage(msg: WsMessage): string {
  // TODO(protobuf): replace with protobuf.serialize(msg)
  return JSON.stringify(msg)
}

function decodeMessage(raw: string): WsMessage | undefined {
  // TODO(protobuf): replace with protobuf.deserialize(raw)
  try {
    return JSON.parse(raw) as WsMessage
  } catch {
    return undefined
  }
}

let ws: WebSocket | null = null
let reconnectDelay = 1000
let reconnectAttempts = 0
const MAX_RECONNECT_ATTEMPTS = 20
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let reqIdSeq = 0
const pending = new Map<
  number,
  {
    resolve: (value: unknown) => void
    reject: (reason: Error) => void
    timer: ReturnType<typeof setTimeout>
  }
>()
const handlers = new Map<WsMessageType, Set<(msg: WsMessage, reqId?: number) => void>>()
let intentionalClose = false
let connectingPromise: Promise<void> | null = null

export type WsStatus = 'closed' | 'connecting' | 'open'

let currentStatus: WsStatus = 'closed'
const statusListeners = new Set<(s: WsStatus) => void>()

function setStatus(status: WsStatus) {
  currentStatus = status
  statusListeners.forEach((fn) => fn(status))
}

export function getStatus(): WsStatus {
  return currentStatus
}

export function onStatusChange(fn: (s: WsStatus) => void): () => void {
  statusListeners.add(fn)
  return () => statusListeners.delete(fn)
}

function rejectPending(reason: Error) {
  for (const p of pending.values()) {
    clearTimeout(p.timer)
    p.reject(reason)
  }
  pending.clear()
}

function cleanupSocket() {
  if (ws) {
    ws.onopen = null
    ws.onmessage = null
    ws.onclose = null
    ws.onerror = null
    try {
      ws.close()
    } catch {}
    ws = null
  }
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
}

function tryReconnect() {
  if (intentionalClose) return
  if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
    console.error(`WS reconnect limit (${MAX_RECONNECT_ATTEMPTS}) reached, giving up`)
    return
  }
  const delay = reconnectDelay * (0.8 + Math.random() * 0.4)
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null
    reconnectAttempts++
    connectInternal().catch(() => {})
  }, delay)
  reconnectDelay = Math.min(reconnectDelay * 2, 30000)
}

function connectInternal(): Promise<void> {
  if (connectingPromise) return connectingPromise

  let promise: Promise<void>
  promise = new Promise((resolve, reject) => {
    if (ws?.readyState === WebSocket.OPEN) {
      resolve()
      return
    }
    if (ws?.readyState === WebSocket.CONNECTING) {
      const startAt = Date.now()
      const check = () => {
        if (ws?.readyState === WebSocket.OPEN) {
          resolve()
          return
        }
        if (!ws || ws.readyState === WebSocket.CLOSED) {
          reject(new Error('Connection failed'))
          return
        }
        if (Date.now() - startAt > 10000) {
          reject(new Error('Connection timeout'))
          return
        }
        setTimeout(check, 50)
      }
      check()
      return
    }

    setStatus('connecting')
    const socket = new WebSocket(getWsUrl())
    ws = socket

    socket.onopen = () => {
      intentionalClose = false
      reconnectAttempts = 0
      reconnectDelay = 1000
      setStatus('open')
      resolve()
    }

    socket.onmessage = (event) => {
      const msg = decodeMessage(String(event.data))
      if (!msg) return
      const reqId = typeof msg.req_id === 'number' ? msg.req_id : undefined
      if (reqId !== undefined && pending.has(reqId)) {
        const p = pending.get(reqId)!
        clearTimeout(p.timer)
        pending.delete(reqId)
        if (msg.type === WsMessageType.ERROR) {
          p.reject(new Error(String(msg.message || 'Server error')))
        } else {
          p.resolve(msg)
        }
        return
      }
      const type = (msg.type as WsMessageType) || WsMessageType.UNKNOWN
      if (type && handlers.has(type)) {
        handlers.get(type)!.forEach((h) => h(msg, reqId))
      }
    }

    socket.onclose = () => {
      setStatus('closed')
      ws = null
      rejectPending(new Error('Connection closed'))
      tryReconnect()
    }

    socket.onerror = () => {
      reject(new Error('WebSocket error'))
    }
  })

  connectingPromise = promise
  promise.finally(() => {
    connectingPromise = null
  })

  return promise
}

export function connect(): Promise<void> {
  return connectInternal()
}

export function disconnect(): void {
  intentionalClose = true
  cleanupSocket()
  rejectPending(new Error('Disconnected'))
}

export async function send(type: WsMessageType, payload: WsPayload = {}): Promise<void> {
  await connect()
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    throw new Error('WebSocket is not open')
  }
  ws.send(encodeMessage({ type, ...payload }))
}

export async function sendAndWait(
  type: WsMessageType,
  payload: WsPayload = {},
  _responseType: WsMessageType,
  timeoutMs = 10000,
): Promise<WsMessage> {
  await connect()
  return new Promise((resolve, reject) => {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      reject(new Error('WebSocket is not open'))
      return
    }
    const reqId = ++reqIdSeq
    const timer = setTimeout(() => {
      pending.delete(reqId)
      reject(new Error(`Request timeout: ${type}`))
    }, timeoutMs)

    pending.set(reqId, {
      resolve: resolve as (value: unknown) => void,
      reject,
      timer,
    })

    ws.send(encodeMessage({ type, ...payload, req_id: reqId }))
  })
}

export function onMessage(
  type: WsMessageType,
  handler: (msg: WsMessage, reqId?: number) => void,
): () => void {
  if (!handlers.has(type)) {
    handlers.set(type, new Set())
  }
  handlers.get(type)!.add(handler)
  return () => {
    handlers.get(type)?.delete(handler)
  }
}
