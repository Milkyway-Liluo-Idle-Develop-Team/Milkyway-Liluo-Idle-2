import { test, expect } from './fixtures'

/** Scan all skill tabs and scene buttons to find the first executable event card.
 *  Returns the Locator of the card, or null if none found. */
async function findExecutableEvent(
  page: import('@playwright/test').Page,
): Promise<import('@playwright/test').Locator | null> {
  const skills = page.locator('.skill-row')
  const skillCount = await skills.count()

  for (let s = 0; s < skillCount; s++) {
    await skills.nth(s).click()
    await page.waitForTimeout(300)

    const scenes = page.locator('.scene-btn')
    const sceneCount = await scenes.count()

    const scanCards = async () => {
      const cards = page.locator('.event-card')
      const count = await cards.count()
      for (let i = 0; i < count; i++) {
        const card = cards.nth(i)
        const tagText = await card.locator('.tag').textContent()
        if (tagText?.includes('可执行')) {
          return card
        }
      }
      return null
    }

    if (sceneCount === 0) {
      const found = await scanCards()
      if (found) return found
      continue
    }

    for (let m = 0; m < sceneCount; m++) {
      await scenes.nth(m).click()
      await page.waitForTimeout(300)
      const found = await scanCards()
      if (found) return found
    }
  }
  return null
}

/** Scan all skill tabs and scene buttons to find the first executable loop event
 *  that has a +队列 button. Returns the Locator of the card, or null. */
async function findExecutableLoopEvent(
  page: import('@playwright/test').Page,
): Promise<import('@playwright/test').Locator | null> {
  const skills = page.locator('.skill-row')
  const skillCount = await skills.count()

  for (let s = 0; s < skillCount; s++) {
    await skills.nth(s).click()
    await page.waitForTimeout(300)

    const scenes = page.locator('.scene-btn')
    const sceneCount = await scenes.count()

    const scanCards = async () => {
      const cards = page.locator('.event-card')
      const count = await cards.count()
      for (let i = 0; i < count; i++) {
        const card = cards.nth(i)
        const hasQueueBtn = (await card.locator('.event-action.secondary').count()) > 0
        if (!hasQueueBtn) continue
        const tagText = await card.locator('.tag').textContent()
        if (tagText?.includes('可执行')) {
          return card
        }
      }
      return null
    }

    if (sceneCount === 0) {
      const found = await scanCards()
      if (found) return found
      continue
    }

    for (let m = 0; m < sceneCount; m++) {
      await scenes.nth(m).click()
      await page.waitForTimeout(300)
      const found = await scanCards()
      if (found) return found
    }
  }
  return null
}

test.describe('Game UI interactions', () => {
  test('page loads with skill tabs, scene buttons and event cards after navigation', async ({ authPage }) => {
    const { page } = authPage

    await expect(page.locator('.skill-row').first()).toBeVisible({ timeout: 10000 })
    await expect(page.locator('.scene-btn').first()).toBeVisible({ timeout: 5000 })

    const card = await findExecutableEvent(page)
    if (!card) {
      test.skip(true, 'No executable events visible in any skill/scene combination')
      return
    }

    await expect(card).toBeVisible({ timeout: 5000 })
  })

  test('click execute button on first executable event and receive state diff', async ({ authPage }) => {
    const { page, ws } = authPage

    const messages: Array<{ type: string }> = []
    const handler = (f: any) => {
      try {
        const msg = JSON.parse(f.payload as string)
        messages.push({ type: msg.type })
      } catch {
        // ignore non-json frames
      }
    }
    ws.on('framereceived', handler)

    await page.waitForSelector('.skill-row', { timeout: 10000 })
    const card = await findExecutableEvent(page)
    if (!card) {
      test.skip(true, 'No executable events visible for this test user')
      return
    }

    const btn = card.locator('.event-action').first()
    await btn.click()

    // Wait for backend tick to process command and push diff
    await page.waitForTimeout(3000)
    ws.off('framereceived', handler)

    console.log('WS messages (UI execute):', messages.map((m) => m.type))
    const stateMessages = messages.filter((m) => m.type === 'state.diff' || m.type === 'state.full')
    expect(stateMessages.length).toBeGreaterThan(0)
  })

  test('click +队列 on a loop event and verify queue panel', async ({ authPage }) => {
    const { page, ws } = authPage

    const messages: Array<{ type: string }> = []
    const handler = (f: any) => {
      try {
        const msg = JSON.parse(f.payload as string)
        messages.push({ type: msg.type })
      } catch {
        // ignore non-json frames
      }
    }
    ws.on('framereceived', handler)

    await page.waitForSelector('.skill-row', { timeout: 10000 })
    const card = await findExecutableLoopEvent(page)
    if (!card) {
      test.skip(true, 'No executable loop events available for queue append')
      return
    }

    const queueBtn = card.locator('.event-action.secondary')
    await queueBtn.click()

    await page.waitForTimeout(3000)
    ws.off('framereceived', handler)

    console.log('WS messages (UI queue):', messages.map((m) => m.type))
    const stateMessages = messages.filter((m) => m.type === 'state.diff' || m.type === 'state.full')
    expect(stateMessages.length).toBeGreaterThan(0)

    // Queue panel should now be visible with exactly 1 item
    const queueBar = page.locator('.queue-bar')
    await expect(queueBar).toBeVisible({ timeout: 5000 })
    await expect(queueBar.locator('.queue-item')).toHaveCount(1)
  })

  test('switch skill tab updates event list', async ({ authPage }) => {
    const { page } = authPage

    await page.waitForSelector('.skill-row', { timeout: 10000 })

    // First, navigate to a skill tab that shows events
    const card = await findExecutableEvent(page)
    if (!card) {
      test.skip(true, 'No executable events visible in any skill/scene combination')
      return
    }

    const firstEventNames = await page.locator('.event-card .event-head strong').allTextContents()
    expect(firstEventNames.length).toBeGreaterThan(0)

    // Try clicking another skill tab
    const skillRows = page.locator('.skill-row')
    const skillCount = await skillRows.count()
    let changed = false

    for (let i = 0; i < skillCount; i++) {
      await skillRows.nth(i).click()
      await page.waitForTimeout(500)
      const currentNames = await page.locator('.event-card .event-head strong').allTextContents()
      const differs =
        currentNames.length !== firstEventNames.length ||
        currentNames.some((name, idx) => name !== firstEventNames[idx])
      if (differs) {
        changed = true
        break
      }
    }

    // If we couldn't find a different tab, try clicking scene buttons instead
    if (!changed) {
      const sceneBtns = page.locator('.scene-btn')
      const sceneCount = await sceneBtns.count()
      for (let i = 0; i < sceneCount; i++) {
        await sceneBtns.nth(i).click()
        await page.waitForTimeout(500)
        const currentNames = await page.locator('.event-card .event-head strong').allTextContents()
        const differs =
          currentNames.length !== firstEventNames.length ||
          currentNames.some((name, idx) => name !== firstEventNames[idx])
        if (differs) {
          changed = true
          break
        }
      }
    }

    expect(changed).toBe(true)
  })
})
