import { reactive } from 'vue'
import { apiUrl, postJson, getJson } from './api'

let cachedAuth: { ok: boolean; checkedAt: number } | null = null

export function clearAuthCache() {
  cachedAuth = null
}

export const userProfile = reactive<{
  uid: number | null
  username: string
  email: string
  created_at: number
}>({
  uid: null,
  username: '',
  email: '',
  created_at: 0,
})

export function setUserProfile(p: { uid: number; username: string; email: string; created_at: number }) {
  userProfile.uid = p.uid
  userProfile.username = p.username
  userProfile.email = p.email
  userProfile.created_at = p.created_at
}

export async function isAuthenticated(): Promise<boolean> {
  if (cachedAuth && Date.now() - cachedAuth.checkedAt < 30_000) return cachedAuth.ok

  try {
    const res = await getJson<{ id: number; username: string; email: string; created_at: number }>('/api/v1/auth/me', {
      credentials: 'include',
    })
    const ok = res.ok
    if (ok) {
      setUserProfile({
        uid: res.data.id,
        username: res.data.username,
        email: res.data.email,
        created_at: res.data.created_at,
      })
    }
    cachedAuth = { ok, checkedAt: Date.now() }
    return ok
  } catch {
    return false
  }
}

export async function logout(): Promise<{ ok: true } | { ok: false; error: string }> {
  const res = await postJson<Record<string, unknown>>('/api/v1/auth/logout', {}, { credentials: 'include' })
  clearAuthCache()
  if (!res.ok) return { ok: false, error: res.error }
  return { ok: true }
}
