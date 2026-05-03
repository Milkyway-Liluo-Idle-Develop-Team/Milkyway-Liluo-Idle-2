const defaultBaseUrl = 'http://localhost:8080'

export const API_BASE_URL: string =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) || defaultBaseUrl

export function apiUrl(path: string) {
  if (!path) return API_BASE_URL
  if (path.startsWith('http://') || path.startsWith('https://')) return path
  if (path.startsWith('/')) return `${API_BASE_URL}${path}`
  return `${API_BASE_URL}/${path}`
}

type ApiError = {
  success?: false
  error?: string
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

    const data = (await res.json().catch(() => ({}))) as TResponse & ApiError
    if (!res.ok || data?.success === false) {
      return {
        ok: false,
        status: res.status,
        error: data?.error || `HTTP ${res.status}`,
      }
    }
    return { ok: true, data }
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

    const data = (await res.json().catch(() => ({}))) as TResponse & ApiError
    if (!res.ok || data?.success === false) {
      return {
        ok: false,
        status: res.status,
        error: data?.error || `HTTP ${res.status}`,
      }
    }
    return { ok: true, data }
  } catch (err: any) {
    return { ok: false, status: 0, error: err?.message || '网络请求失败' }
  }
}
