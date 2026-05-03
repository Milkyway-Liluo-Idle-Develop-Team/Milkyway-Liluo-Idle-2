import { API_BASE_URL } from './api'

function getWsUrl(): string {
  const base = API_BASE_URL
  return base.replace(/^http/, 'ws') + '/ws'
}

export interface WsEnvelope {
  id?: string
  type: string
  payload?: Record<string, unknown>
  error?: { code: string; message: string; fields?: Record<string, string> }
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
const handlers = new Map<string, Set<(msg: WsEnvelope) => void>>()
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
      let msg: WsEnvelope
      try {
        msg = JSON.parse(String(event.data)) as WsEnvelope
      } catch {
        return
      }
      const reqId = msg.id ? parseInt(msg.id, 10) : NaN
      if (!isNaN(reqId) && pending.has(reqId)) {
        const p = pending.get(reqId)!
        clearTimeout(p.timer)
        pending.delete(reqId)
        if (msg.error) {
          p.reject(new Error(msg.error.message || 'Server error'))
        } else {
          p.resolve(msg)
        }
        return
      }
      const type = msg.type || 'unknown'
      if (type && handlers.has(type)) {
        handlers.get(type)!.forEach((h) => h(msg))
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

export async function send(type: string, payload: Record<string, unknown> = {}): Promise<void> {
  await connect()
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    throw new Error('WebSocket is not open')
  }
  ws.send(JSON.stringify({ type, ...payload }))
}

export async function sendAndWait(
  type: string,
  payload: Record<string, unknown> = {},
  timeoutMs = 10000,
): Promise<WsEnvelope> {
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

    ws.send(JSON.stringify({ type, ...payload, id: String(reqId) }))
  })
}

export function onMessage(type: string, handler: (msg: WsEnvelope) => void): () => void {
  if (!handlers.has(type)) {
    handlers.set(type, new Set())
  }
  handlers.get(type)!.add(handler)
  return () => {
    handlers.get(type)?.delete(handler)
  }
}
