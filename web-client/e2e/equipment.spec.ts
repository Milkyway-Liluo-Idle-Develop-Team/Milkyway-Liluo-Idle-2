import { test, expect } from './fixtures'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Call the test-only API to add items directly to a user's inventory. */
async function addInventory(
  page: import('@playwright/test').Page,
  username: string,
  items: Array<{ item_id: number; item_state?: number; quantity: number }>,
) {
  const res = await page.request.post('http://localhost:8080/api/v1/test/add-inventory', {
    data: {
      username,
      items: items.map((it) => ({
        item_id: it.item_id,
        item_state: it.item_state ?? 0,
        quantity: it.quantity,
      })),
    },
  })
  expect(res.ok(), `add-inventory failed: ${await res.text().catch(() => '')}`).toBe(true)
}

/** Refresh the page and wait for WS + game state to settle. Returns the new WebSocket. */
async function refreshAndWait(page: import('@playwright/test').Page) {
  const wsPromise = page.waitForEvent('websocket', {
    timeout: 15000,
    predicate: (ws) => ws.url().includes('/ws'),
  })
  await page.reload()
  const ws = await wsPromise
  await page.waitForFunction(() => {
    const s = (window as any).__gameStore
    return s && Object.keys(s.state.skills).length > 0
  }, { timeout: 15000 })
  // Extra grace for state.full to arrive and static config to resolve.
  await page.waitForTimeout(800)
  return ws
}

/** Switch the right panel to the Equipment tab. */
async function openEquipmentTab(page: import('@playwright/test').Page) {
  const tab = page.locator('.right-tab-btn', { hasText: '装备' })
  await tab.click()
  await page.waitForTimeout(300)
}

/** Find an equipment slot cell by its data-slot-id (e.g. "main_hand", "felling"). */
async function findSlotById(
  page: import('@playwright/test').Page,
  slotId: string,
): Promise<import('@playwright/test').Locator | null> {
  const cell = page.locator(`.equip-slot-cell[data-slot-id="${slotId}"]`)
  if (await cell.count() === 0) return null
  return cell
}

/** Read the current equipment/tools state from the store. */
async function readStoreEquipment(page: import('@playwright/test').Page) {
  return page.evaluate(() => {
    const store = (window as any).__gameStore
    return {
      equipment: { ...(store.state.equipment || {}) },
      tools: { ...(store.state.tools || {}) },
      inventory: (store.state.inventory || []).map((it: any) => ({ id: it.id, qty: it.qty })),
      attributes: { ...(store.state.attributes || {}) },
    }
  })
}

// ---------------------------------------------------------------------------
// Test constants (numeric IDs from id_registry.json)
// ---------------------------------------------------------------------------
const ITEMS = {
  wooden_sword: 35,
  wooden_axe: 30,
  leather_boots: 15,
  leather_helmet: 17,
  leather_breastplate: 16,
  leather_legarmor: 18,
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Equipment equip / unequip', () => {
  test.setTimeout(60000)

  test('diagnostic: direct equip via store action', async ({ authPage }) => {
    const { page, username } = authPage
    await addInventory(page, username, [{ item_id: ITEMS.wooden_sword, quantity: 1 }])
    await refreshAndWait(page)

    const result = await page.evaluate(async () => {
      const store = (window as any).__gameStore
      const actions = (window as any).__actions

      try {
        await actions.equipItem('wooden_sword', 'main_hand')
        await new Promise((r) => setTimeout(r, 3000))
        return {
          error: store.actionError,
          equipment: store.state.equipment,
          inventory: store.state.inventory.filter((it: any) => it.id === 'wooden_sword'),
          attributes: store.state.attributes,
        }
      } catch (e: any) {
        return { jsError: String(e?.message || e) }
      }
    })

    console.log('Diagnostic result:', JSON.stringify(result, null, 2))
  })

  test('equip battle equipment via UI and unequip', async ({ authPage, context }) => {
    const { page, username } = authPage

    // 1. Give the player a sword and boots directly.
    await addInventory(page, username, [
      { item_id: ITEMS.wooden_sword, quantity: 1 },
      { item_id: ITEMS.leather_boots, quantity: 1 },
    ])
    const ws = await refreshAndWait(page)

    // 2. Switch to Equipment tab.
    await openEquipmentTab(page)

    // 3. Click the main_hand slot.
    const mainHandSlot = await findSlotById(page, 'main_hand')
    expect(mainHandSlot, 'main_hand slot should be visible').not.toBeNull()
    await mainHandSlot!.click()

    // 4. Slot picker should open with the sword as a candidate.
    const slotPicker = page.locator('.slot-picker')
    await expect(slotPicker).toBeVisible({ timeout: 5000 })

    // Click the sword candidate (identified by icon src).
    const swordCandidate = slotPicker.locator('.square-cell', {
      has: page.locator('img[src="/icons/items/wooden_sword.svg"]'),
    })
    await expect(swordCandidate).toBeVisible({ timeout: 5000 })

    // Collect WS messages so we can wait for the diff.
    const messages: Array<{ type: string }> = []
    const handler = (f: { payload: string | Buffer }) => {
      try {
        const msg = JSON.parse(f.payload as string)
        messages.push({ type: msg.type })
      } catch {
        /* ignore non-json */
      }
    }
    ws.on('framereceived', handler)

    await swordCandidate.click()
    await page.waitForTimeout(2500)
    ws.off('framereceived', handler)

    // 5. Verify a state diff arrived.
    const diffs = messages.filter((m) => m.type === 'state.diff')
    expect(diffs.length, 'should receive state.diff after equip').toBeGreaterThan(0)

    // 6. Close picker, then verify UI: the main_hand slot shows the sword icon.
    const closeBtn = slotPicker.locator('button', { hasText: '返回' })
    if (await closeBtn.isVisible().catch(() => false)) {
      await closeBtn.click()
      await page.waitForTimeout(200)
    }
    const mainHandAfter = await findSlotById(page, 'main_hand')
    expect(mainHandAfter).not.toBeNull()
    const swordImg = mainHandAfter!.locator('img[src="/icons/items/wooden_sword.svg"]')
    await expect(swordImg).toBeVisible({ timeout: 5000 })

    // 7. Verify store state.
    let storeState = await readStoreEquipment(page)
    expect(storeState.equipment['main_hand']).toBe('wooden_sword')
    expect(storeState.inventory.find((it: any) => it.id === 'wooden_sword')?.qty ?? 0).toBe(0)

    // 8. Verify attribute increased (physical_power base = 10, sword adds 10 => 20).
    expect(storeState.attributes['physical_power']).toBeGreaterThanOrEqual(20)

    // 9. Unequip.
    await mainHandAfter!.click()
    await expect(slotPicker).toBeVisible({ timeout: 5000 })
    const unequipBtn = slotPicker.locator('button', { hasText: '卸下装备' })
    await unequipBtn.click()
    await page.waitForTimeout(2500)

    // 10. Verify UI: main_hand is empty again.
    const mainHandEmpty = await findSlotById(page, 'main_hand')
    expect(mainHandEmpty).not.toBeNull()
    await expect(mainHandEmpty!.locator('img')).not.toBeVisible()

    // 11. Verify store state: item returned to inventory.
    storeState = await readStoreEquipment(page)
    expect(storeState.equipment['main_hand']).toBeUndefined()
    const swordInv = storeState.inventory.find((it: any) => it.id === 'wooden_sword')
    expect(swordInv?.qty ?? 0).toBe(1)

    // 12. Attribute should return to baseline.
    expect(storeState.attributes['physical_power']).toBe(10)
  })

  test('equip tool via UI', async ({ authPage }) => {
    const { page, username } = authPage

    // 1. Give the player an axe.
    await addInventory(page, username, [{ item_id: ITEMS.wooden_axe, quantity: 1 }])
    await refreshAndWait(page)

    // 2. Open Equipment tab.
    await openEquipmentTab(page)

    // 3. Click the felling tool slot.
    const fellingSlot = await findSlotById(page, 'felling')
    expect(fellingSlot, 'felling slot should be visible').not.toBeNull()
    await fellingSlot!.click()

    // 4. Slot picker opens; equip the axe.
    const slotPicker = page.locator('.slot-picker')
    await expect(slotPicker).toBeVisible({ timeout: 5000 })

    const axeCandidate = slotPicker.locator('.square-cell', {
      has: page.locator('img[src="/icons/items/wooden_axe.svg"]'),
    })
    await expect(axeCandidate).toBeVisible({ timeout: 5000 })
    await axeCandidate.click()
    await page.waitForTimeout(2500)

    // 5. Close picker, then verify UI and state.
    const closeBtn2 = slotPicker.locator('button', { hasText: '返回' })
    if (await closeBtn2.isVisible().catch(() => false)) {
      await closeBtn2.click()
      await page.waitForTimeout(200)
    }
    const storeState = await readStoreEquipment(page)
    expect(storeState.tools['felling']).toBe('wooden_axe')

    const fellingAfter = await findSlotById(page, 'felling')
    expect(fellingAfter).not.toBeNull()
    await expect(fellingAfter!.locator('img[src="/icons/items/wooden_axe.svg"]')).toBeVisible({ timeout: 5000 })
    expect(storeState.inventory.find((it: any) => it.id === 'wooden_axe')?.qty ?? 0).toBe(0)

    // felling_production_multiplier should now be > 0 (axe gives 0.1).
    expect(storeState.attributes['felling_production_multiplier']).toBeGreaterThan(0)
  })

  test('equipment persists after refresh and re-login', async ({ authPage, context }) => {
    const { page, username } = authPage

    // 1. Give items and equip sword via direct action (faster than UI clicks).
    await addInventory(page, username, [{ item_id: ITEMS.wooden_sword, quantity: 1 }])
    await refreshAndWait(page)

    // Equip via store action directly.
    await page.evaluate(async () => {
      const store = (window as any).__gameStore
      await store.equipItem('wooden_sword', 'main_hand')
    })
    await page.waitForTimeout(2500)

    // Verify equipped.
    let stateBefore = await readStoreEquipment(page)
    expect(stateBefore.equipment['main_hand']).toBe('wooden_sword')

    // 2. Evict backend session (simulate grace expire).
    const evict = await context.request.post('http://localhost:8080/api/v1/test/evict-session', {
      data: { username },
    })
    expect(evict.ok() || evict.status() === 404).toBe(true)

    // 3. Refresh page.
    await refreshAndWait(page)

    // 4. Verify still equipped after reconnect.
    const stateAfter = await readStoreEquipment(page)
    expect(stateAfter.equipment['main_hand']).toBe('wooden_sword')
    expect(stateAfter.attributes['physical_power']).toBeGreaterThanOrEqual(20)

    // 5. Verify UI still shows the sword.
    await openEquipmentTab(page)
    const mainHandSlot = await findSlotById(page, 'main_hand')
    expect(mainHandSlot).not.toBeNull()
    await expect(mainHandSlot!.locator('img[src="/icons/items/wooden_sword.svg"]')).toBeVisible({ timeout: 5000 })
  })

  test('equip multiple battle slots simultaneously', async ({ authPage }) => {
    const { page, username } = authPage

    // 1. Give a full set of leather armour + sword.
    await addInventory(page, username, [
      { item_id: ITEMS.wooden_sword, quantity: 1 },
      { item_id: ITEMS.leather_helmet, quantity: 1 },
      { item_id: ITEMS.leather_breastplate, quantity: 1 },
      { item_id: ITEMS.leather_legarmor, quantity: 1 },
      { item_id: ITEMS.leather_boots, quantity: 1 },
    ])
    await refreshAndWait(page)

    // 2. Equip each piece via UI.
    await openEquipmentTab(page)

    const slots = [
      { slotId: 'main_hand', item: 'wooden_sword' },
      { slotId: 'head', item: 'leather_helmet' },
      { slotId: 'chest', item: 'leather_breastplate' },
      { slotId: 'leg', item: 'leather_legarmor' },
      { slotId: 'feet', item: 'leather_boots' },
    ]

    for (const { slotId, item } of slots) {
      const cell = await findSlotById(page, slotId)
      expect(cell, `${slotId} slot should exist`).not.toBeNull()
      await cell!.click()

      const picker = page.locator('.slot-picker')
      await expect(picker).toBeVisible({ timeout: 5000 })

      const candidate = picker.locator('.square-cell', {
        has: page.locator(`img[src="/icons/items/${item}.svg"]`),
      })
      if ((await candidate.count()) === 0) {
        // Close picker and skip if candidate not found (should not happen).
        const closeBtn = picker.locator('button', { hasText: '返回' })
        await closeBtn.click()
        await page.waitForTimeout(200)
        continue
      }
      await candidate.click()
      await page.waitForTimeout(800)

      // Close picker if still open (some clicks may not auto-close).
      const closeBtn = picker.locator('button', { hasText: '返回' })
      if (await closeBtn.isVisible().catch(() => false)) {
        await closeBtn.click()
        await page.waitForTimeout(200)
      }
    }

    // 3. Verify all slots equipped in store state.
    const storeState = await readStoreEquipment(page)
    for (const { slotId, item } of slots) {
      expect(storeState.equipment[slotId], `${slotId} should have ${item}`).toBe(item)
    }

    // 4. Verify none of the equipped items remain in inventory.
    for (const { item } of slots) {
      const qty = storeState.inventory.find((it: any) => it.id === item)?.qty ?? 0
      expect(qty, `${item} should be removed from inventory`).toBe(0)
    }

    // 5. Verify physical_power increased (sword adds 10 on top of base 10).
    expect(storeState.attributes['physical_power']).toBeGreaterThanOrEqual(20)
  })
})
