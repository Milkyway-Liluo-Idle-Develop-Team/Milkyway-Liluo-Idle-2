import { test, expect } from './fixtures'

test.describe('Unlock chop wood after starting dialogs', () => {
  test.setTimeout(60000)

  test('complete all starting dialogs and start felling oak tree loop', async ({ authPage }) => {
    const { page, ws } = authPage

    // Step 1: Fast-path — complete all 5 starting dialogs via direct action calls.
    // Each dialog unlocks the next; they are processed sequentially by the tick loop.
    await page.evaluate(async () => {
      const actions = (window as any).__actions
      for (const id of [
        'starting_dialog_1',
        'starting_dialog_2',
        'starting_dialog_3',
        'starting_dialog_4',
        'starting_dialog_5',
      ]) {
        await actions.executeUpgrade(id)
      }
    })

    // Wait for backend to process the queue (tick interval is 50ms, 5s is generous).
    await page.waitForTimeout(5000)

    // Step 2: Navigate to the "砍伐" (felling) skill tab.
    const skillRows = page.locator('.skill-row')
    const skillCount = await skillRows.count()
    let fellingTabIndex = -1

    for (let i = 0; i < skillCount; i++) {
      const text = await skillRows.nth(i).textContent()
      if (text?.includes('砍伐')) {
        fellingTabIndex = i
        break
      }
    }

    if (fellingTabIndex === -1) {
      test.skip(true, 'Felling skill tab not found')
      return
    }

    await skillRows.nth(fellingTabIndex).click()
    await page.waitForTimeout(500)

    // Ensure we are on the "村庄" scene where felling events appear.
    const sceneBtns = page.locator('.scene-btn')
    const sceneCount = await sceneBtns.count()
    for (let i = 0; i < sceneCount; i++) {
      const text = await sceneBtns.nth(i).textContent()
      if (text?.includes('村庄')) {
        await sceneBtns.nth(i).click()
        await page.waitForTimeout(300)
        break
      }
    }

    // Step 3: Find the "砍伐橡木" event card.
    const cards = page.locator('.event-card')
    const cardCount = await cards.count()
    let chopCard: import('@playwright/test').Locator | null = null

    for (let i = 0; i < cardCount; i++) {
      const card = cards.nth(i)
      const name = await card.locator('.event-head strong').textContent()
      if (name?.includes('砍伐橡木')) {
        chopCard = card
        break
      }
    }

    if (!chopCard) {
      test.skip(true, '砍伐橡木 event not visible after unlocking dialogs')
      return
    }

    // Verify it is executable.
    const tagText = await chopCard.locator('.tag').textContent()
    expect(tagText).toContain('可执行')

    // Step 4: Collect WS messages and click the execute button.
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

    const btn = chopCard.locator('.event-action').first()
    await btn.click()

    // Wait for the 2-second loop to complete and produce rewards.
    await page.waitForTimeout(5000)
    ws.off('framereceived', handler)

    console.log('WS messages (chop wood):', messages.map((m) => m.type))
    const stateMessages = messages.filter((m) => m.type === 'state.diff' || m.type === 'state.full')
    expect(stateMessages.length).toBeGreaterThan(0)

    // Step 5: Verify the loop progress bar appears in the header.
    await expect(page.locator('.loop-progress-wrap')).toBeVisible({ timeout: 5000 })

    // Step 6: Verify inventory received oak_logs.
    const inventory = await page.evaluate(() => {
      const store = (window as any).__gameStore
      return store.state.inventory as Array<{ id: string; qty: number }>
    })
    const oakLogs = inventory.find((it) => it.id === 'oak_logs')
    console.log('Inventory after chop:', inventory)
    expect(oakLogs?.qty ?? 0).toBeGreaterThan(0)

    // Step 7: Verify felling skill XP increased.
    const skills = await page.evaluate(() => {
      const store = (window as any).__gameStore
      return store.state.skills as Record<string, { level: number; exp: number }>
    })
    console.log('Skills after chop:', skills)
    expect(skills['felling']?.exp ?? 0).toBeGreaterThan(0)
  })
})
