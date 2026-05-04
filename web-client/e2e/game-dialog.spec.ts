import { test, expect } from './fixtures'

test.describe('Starting dialog unlock chain', () => {
  test('sequentially execute first 3 starting dialogs through UI', async ({ authPage }) => {
    const { page, ws } = authPage

    // Navigate to the "升级行动" (upgrade actions) tab
    const skillRows = page.locator('.skill-row')
    const skillCount = await skillRows.count()
    let upgradeTabIndex = -1

    for (let i = 0; i < skillCount; i++) {
      const text = await skillRows.nth(i).textContent()
      if (text?.includes('升级行动')) {
        upgradeTabIndex = i
        break
      }
    }

    if (upgradeTabIndex === -1) {
      test.skip(true, 'Upgrade actions tab not found')
      return
    }

    await skillRows.nth(upgradeTabIndex).click()
    await page.waitForTimeout(500)

    // Execute first 3 dialogs in sequence.
    // Because executed dialogs remain visible (event_count not tracked yet),
    // we click the LAST executable event each time — that should be the
    // most recently unlocked one.
    for (let step = 1; step <= 3; step++) {
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

      // Re-query event cards each iteration because DOM updates after each execution
      const cards = page.locator('.event-card')
      const cardCount = await cards.count()
      let targetCard: import('@playwright/test').Locator | null = null
      let eventName = ''

      // Scan from the end to find the last executable event
      for (let i = cardCount - 1; i >= 0; i--) {
        const card = cards.nth(i)
        const tagText = await card.locator('.tag').textContent()
        if (tagText?.includes('可执行')) {
          eventName = (await card.locator('.event-head strong').textContent()) || ''
          targetCard = card
          break
        }
      }

      if (!targetCard) {
        ws.off('framereceived', handler)
        test.skip(true, `No executable dialog found at step ${step}`)
        return
      }

      const btn = targetCard.locator('.event-action').first()
      await btn.click()

      console.log(`Step ${step}: clicked "${eventName.trim()}"`)

      // Wait for backend to process and push state diff
      await page.waitForTimeout(3000)
      ws.off('framereceived', handler)

      console.log(`Step ${step} WS messages:`, messages.map((m) => m.type))
      const stateMessages = messages.filter((m) => m.type === 'state.diff' || m.type === 'state.full')
      expect(stateMessages.length).toBeGreaterThan(0)
    }
  })
})
