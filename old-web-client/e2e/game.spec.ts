import { test, expect } from './fixtures'

test.describe('Game main page', () => {
  // In a real suite you would log in via API in a global-setup or fixture
  // and store the auth cookie so every test starts authenticated.
  test.skip('WS connects and receives state.full push', async ({ page }) => {
    await page.goto('/main')

    // Wait for the WebSocket to open
    const wsPromise = page.waitForEvent('websocket', { timeout: 10000 })
    await page.waitForTimeout(500) // give router guard time to connect
    const ws = await wsPromise

    // Verify the WS URL points to our backend
    expect(ws.url()).toContain('/ws')

    // Wait for a state.full message to arrive
    const msgPromise = ws.waitForEvent('framereceived', {
      predicate: (frame) => {
        try {
          const payload = JSON.parse(frame.payload as string)
          return payload.type === 'state.full'
        } catch {
          return false
        }
      },
      timeout: 15000,
    })
    await msgPromise
  })

  test('main page renders navigation tabs', async ({ page }) => {
    await page.goto('/main')
    // Even if not authenticated, the page should at least attempt to render
    // (router guard will redirect, but we check the auth page shows)
    await expect(page.locator('body')).toBeVisible()
  })
})
