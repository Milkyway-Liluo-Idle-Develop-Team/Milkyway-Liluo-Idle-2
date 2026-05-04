import { API_BASE_URL } from './api'
import { Envelope } from '@/pb/envelope'
import type { Error as PbError } from '@/pb/envelope'
import { USE_JSON } from './config'

const WS_URL = API_BASE_URL.replace(/^http/, 'ws') + '/ws'

// ============================================================================
// Connection state
// ============================================================================
let ws: WebSocket | null = null
let reconnectDelay = 1000
let reconnectAttempts = 0
const MAX_RECONNECT_ATTEMPTS = 20
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
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

// ============================================================================
// Request/response correlation
// ============================================================================
const pending = new Map<
  string,
  {
    resolve: (env: Envelope) => void
    reject: (reason: Error) => void
    timer: ReturnType<typeof setTimeout>
  }
>()

function rejectPending(reason: Error) {
  for (const p of pending.values()) {
    clearTimeout(p.timer)
    p.reject(reason)
  }
  pending.clear()
}

// ============================================================================
// Message handlers (broadcast / server-push)
// ============================================================================
export type PushHandler = (type: string, payload: Uint8Array | unknown, envelope: Envelope) => void

const pushHandlers = new Map<string, Set<PushHandler>>()

export function onMessage(type: string, handler: PushHandler): () => void {
  if (!pushHandlers.has(type)) pushHandlers.set(type, new Set())
  pushHandlers.get(type)!.add(handler)
  return () => pushHandlers.get(type)?.delete(handler)
}

// ============================================================================
// Low-level connection
// ============================================================================
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
    console.error(`WS reconnect limit (${MAX_RECONNECT_ATTEMPTS}) reached`)
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

/** Decode incoming frame. JSON mode returns parsed payload as rawPayload; binary mode uses envelope.payload. */
function decodeEnvelope(data: string | ArrayBuffer): { envelope: Envelope; rawPayload?: unknown } {
  if (USE_JSON) {
    const parsed = JSON.parse(data as string)
    return {
      envelope: {
        id: parsed.id ?? '',
        type: parsed.type ?? '',
        payload: undefined,
        error: parsed.error
          ? { code: parsed.error.code ?? '', message: parsed.error.message ?? '', fields: parsed.error.fields }
          : undefined,
      },
      rawPayload: parsed.payload,
    }
  }
  const envelope = Envelope.decode(new Uint8Array(data as ArrayBuffer))
  return { envelope, rawPayload: envelope.payload }
}

function connectInternal(): Promise<void> {
  if (connectingPromise) return connectingPromise

  const promise = new Promise<void>((resolve, reject) => {
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
    const socket = new WebSocket(WS_URL)
    if (!USE_JSON) {
      socket.binaryType = 'arraybuffer'
    }
    ws = socket

    socket.onopen = () => {
      intentionalClose = false
      reconnectAttempts = 0
      reconnectDelay = 1000
      setStatus('open')
      resolve()
    }

    socket.onmessage = (event) => {
      let envelope: Envelope
      let rawPayload: unknown
      try {
        const decoded = decodeEnvelope(event.data as string | ArrayBuffer)
        envelope = decoded.envelope
        rawPayload = decoded.rawPayload
      } catch (e) {
        console.error('WS decode error', e)
        return
      }

      // Correlated response
      if (envelope.id && pending.has(envelope.id)) {
        const p = pending.get(envelope.id)!
        clearTimeout(p.timer)
        pending.delete(envelope.id)
        if (envelope.error) {
          p.reject(new Error(envelope.error.message || 'Server error'))
        } else {
          p.resolve(envelope)
        }
        return
      }

      // Broadcast push
      const type = envelope.type || 'unknown'
      const payload = USE_JSON ? rawPayload : (envelope.payload || new Uint8Array(0))
      const handlers = pushHandlers.get(type)
      if (handlers) {
        handlers.forEach((h) => h(type, payload, envelope))
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

// ============================================================================
// Send helpers — unified interface, transport-layer switch only
// ============================================================================

/** Send a message. `payload` is a JS object in JSON mode, Uint8Array in binary mode. */
export async function send(type: string, payload: unknown): Promise<void> {
  await connect()
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    throw new Error('WebSocket is not open')
  }
  const id = crypto.randomUUID()
  if (USE_JSON) {
    ws.send(JSON.stringify({ id, type, payload }))
  } else {
    ws.send(Envelope.encode({ id, type, payload: payload as Uint8Array }).finish())
  }
}

/** Send and wait for correlated response. */
export async function sendAndWait(
  type: string,
  payload: unknown,
  timeoutMs = 10000,
): Promise<Envelope> {
  await connect()
  return new Promise((resolve, reject) => {
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      reject(new Error('WebSocket is not open'))
      return
    }
    const id = crypto.randomUUID()
    const timer = setTimeout(() => {
      pending.delete(id)
      reject(new Error(`Request timeout: ${type}`))
    }, timeoutMs)

    pending.set(id, { resolve, reject, timer })
    if (USE_JSON) {
      ws.send(JSON.stringify({ id, type, payload }))
    } else {
      ws.send(Envelope.encode({ id, type, payload: payload as Uint8Array }).finish())
    }
  })
}
