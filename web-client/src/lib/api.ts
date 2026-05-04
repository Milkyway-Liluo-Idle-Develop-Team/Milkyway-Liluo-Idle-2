const defaultBaseUrl = 'http://localhost:8080'

export const API_BASE_URL: string =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) || defaultBaseUrl

export function apiUrl(path: string) {
  if (!path) return API_BASE_URL
  if (path.startsWith('http://') || path.startsWith('https://')) return path
  if (path.startsWith('/')) return `${API_BASE_URL}${path}`
  return `${API_BASE_URL}/${path}`
}

// Backend wraps every HTTP JSON response in an envelope:
//   { "data": ... }        // success
//   { "error": { ... } }   // failure
type Envelope<T> =
  | { data: T; error?: undefined }
  | { data?: undefined; error: { code: string; message: string } }

function isEmptyObject(v: unknown): v is Record<string, never> {
  return typeof v === 'object' && v !== null && Object.keys(v).length === 0
}

export async function postJson<TResponse>(
  path: string,
  body: unknown,
  options?: { credentials?: RequestCredentials },
): Promise<{ ok: true; data: TResponse } | { ok: false; status: number; error: string }> {
  try {
    const res = await fetch(apiUrl(path), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      credentials: options?.credentials ?? 'include',
    })

    // 204 No Content or empty body — treat as success
    if (res.ok && (res.status === 204 || res.headers.get('content-length') === '0')) {
      return { ok: true, data: {} as TResponse }
    }

    const payload = (await res.json().catch(() => ({}))) as Envelope<TResponse>

    // Error envelope or bad HTTP status
    if (!res.ok || payload.error) {
      const msg = payload.error?.message || `HTTP ${res.status}`
      return { ok: false, status: res.status, error: msg }
    }

    // Success envelope: { data: ... }
    if (!('data' in payload)) {
      return { ok: false, status: res.status, error: 'invalid response format' }
    }

    return { ok: true, data: payload.data }
  } catch (err: any) {
    return { ok: false, status: 0, error: err?.message || '网络请求失败' }
  }
}

export async function getJson<TResponse>(
  path: string,
  options?: { credentials?: RequestCredentials },
): Promise<{ ok: true; data: TResponse } | { ok: false; status: number; error: string }> {
  try {
    const res = await fetch(apiUrl(path), {
      method: 'GET',
      credentials: options?.credentials ?? 'include',
    })

    if (res.ok && (res.status === 204 || res.headers.get('content-length') === '0')) {
      return { ok: true, data: {} as TResponse }
    }

    const payload = (await res.json().catch(() => ({}))) as Envelope<TResponse>

    if (!res.ok || payload.error) {
      const msg = payload.error?.message || `HTTP ${res.status}`
      return { ok: false, status: res.status, error: msg }
    }

    if (!('data' in payload)) {
      return { ok: false, status: res.status, error: 'invalid response format' }
    }

    return { ok: true, data: payload.data }
  } catch (err: any) {
    return { ok: false, status: 0, error: err?.message || '网络请求失败' }
  }
}
