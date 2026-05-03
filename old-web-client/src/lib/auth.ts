import { reactive } from 'vue'
import { apiUrl, postJson } from './api'

const AUTH_TTL_MS = 30_000

let cachedAuth: { ok: boolean; checkedAt: number } | null = null

export function clearAuthCache() {
  cachedAuth = null
}

// 登录/注册时写入，UserView 直接读取
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
  if (cachedAuth && Date.now() - cachedAuth.checkedAt < AUTH_TTL_MS) return cachedAuth.ok

  try {
    const res = await fetch(apiUrl('/api/heartbeat'), {
      method: 'POST',
      credentials: 'include',
    })
    const ok = res.ok
    cachedAuth = { ok, checkedAt: Date.now() }
    return ok
  } catch {
    // 网络错误时不缓存，避免服务端重启期间把未认证状态锁死 30 秒
    return false
  }
}

export async function logout(): Promise<{ ok: true } | { ok: false; error: string }> {
  const res = await postJson<{ success: true }>('/api/logout', {}, { credentials: 'include' })
  clearAuthCache()
  if (!res.ok) return { ok: false, error: res.error }
  return { ok: true }
}

