import { test as base, expect } from '@playwright/test'

export type AuthFixture = {
  authPage: {
    page: import('@playwright/test').Page
    username: string
    ws: import('@playwright/test').WebSocket
  }
}

export const test = base.extend<AuthFixture>({
  authPage: async ({ page, context }, use) => {
    const username = `e2e_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`
    const password = 'testpass123'
    const email = `${username}@test.com`

    // 1. Register via API (retry once on transient failures)
    let reg = await context.request.post('http://localhost:8080/api/v1/auth/register', {
      data: { username, password, email },
    })
    if (!reg.ok() && reg.status() !== 409) {
      const body = await reg.text().catch(() => '')
      console.warn(`Register failed (${reg.status()}): ${body}. Retrying...`)
      await page.waitForTimeout(500)
      reg = await context.request.post('http://localhost:8080/api/v1/auth/register', {
        data: { username, password, email },
      })
    }
    if (!reg.ok() && reg.status() !== 409) {
      const body = await reg.text().catch(() => '')
      throw new Error(`Register failed: ${reg.status()} ${body}`)
    }

    // 2. Login via API (sets cookie automatically in the context)
    let login = await context.request.post('http://localhost:8080/api/v1/auth/login', {
      data: { username, password },
    })
    if (!login.ok()) {
      const body = await login.text().catch(() => '')
      console.warn(`Login failed (${login.status()}): ${body}. Retrying...`)
      await page.waitForTimeout(500)
      login = await context.request.post('http://localhost:8080/api/v1/auth/login', {
        data: { username, password },
      })
    }
    if (!login.ok()) {
      const body = await login.text().catch(() => '')
      throw new Error(`Login failed: ${login.status()} ${body}`)
    }

    // 3. Navigate to main page as authenticated user
    // Filter out Vite HMR websocket; wait for the app /ws connection.
    const wsPromise = page.waitForEvent('websocket', {
      timeout: 15000,
      predicate: (ws) => ws.url().includes('/ws'),
    })
    await page.goto('/main')
    await page.waitForURL('/main', { timeout: 10000 })
    const ws = await wsPromise
    expect(ws.url()).toContain('/ws')

    // 4. Wait for static config + WS state to settle
    await page.waitForFunction(() => {
      const s = (window as any).__gameStore
      return s && Object.keys(s.state.skills).length > 0
    }, { timeout: 15000 })

    await use({ page, username, ws })

    // 5. Cleanup: delete user entirely
    await context.request.post('http://localhost:8080/api/v1/test/delete-user', {
      data: { username },
    })
  },
})

export { expect } from '@playwright/test'
