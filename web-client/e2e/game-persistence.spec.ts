import { test, expect } from './fixtures'

test.describe('Bestiary persistence across refresh and re-login', () => {
  test.setTimeout(120000)

  test('refresh page preserves discovered items and unlocked crafting events', async ({ authPage, context }) => {
    const { page, username } = authPage

    // 1. 完成起始对话，解锁砍伐
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
    await page.waitForTimeout(3000)

    // 2. 执行砍伐橡木，获得 oak_logs
    await page.evaluate(async () => {
      const actions = (window as any).__actions
      await actions.queueAppend('felling_oak_tree', 3)
      await actions.executeInstant('felling_oak_tree')
    })
    await page.waitForTimeout(6000)

    // 3. 验证制造标签中有制作橡木木板
    const skillRows = page.locator('.skill-row')
    for (let i = 0; i < await skillRows.count(); i++) {
      if ((await skillRows.nth(i).textContent())?.includes('制造')) {
        await skillRows.nth(i).click()
        break
      }
    }
    await page.waitForTimeout(500)

    let foundBeforeRefresh = false
    const cardsBefore = page.locator('.event-card')
    for (let i = 0; i < await cardsBefore.count(); i++) {
      const title = await cardsBefore.nth(i).locator('.event-head strong').textContent()
      if (title?.includes('制作橡木木板')) {
        foundBeforeRefresh = true
        break
      }
    }
    expect(foundBeforeRefresh, '刷新前应能看到制作橡木木板').toBe(true)

    // 4. 强制关闭后端 player session（模拟 grace expire）
    const evict = await context.request.post('http://localhost:8080/api/v1/test/evict-session', {
      data: { username },
    })
    expect(evict.ok() || evict.status() === 404).toBe(true)

    // 5. 刷新页面
    await page.reload()
    await page.waitForTimeout(3000)

    // 6. 等待 WS 连接恢复
    await page.waitForFunction(() => {
      const store = (window as any).__gameStore
      return store && store.state && store.state.inventory.length > 0
    }, { timeout: 15000 })

    // 7. 再次切换到制造标签
    const skillRowsAfter = page.locator('.skill-row')
    for (let i = 0; i < await skillRowsAfter.count(); i++) {
      if ((await skillRowsAfter.nth(i).textContent())?.includes('制造')) {
        await skillRowsAfter.nth(i).click()
        break
      }
    }
    await page.waitForTimeout(500)

    let foundAfterRefresh = false
    const cardsAfter = page.locator('.event-card')
    for (let i = 0; i < await cardsAfter.count(); i++) {
      const title = await cardsAfter.nth(i).locator('.event-head strong').textContent()
      if (title?.includes('制作橡木木板')) {
        foundAfterRefresh = true
        break
      }
    }
    expect(foundAfterRefresh, '刷新后仍应能看到制作橡木木板').toBe(true)

    // 8. 验证 inventory 仍然保留
    const state = await page.evaluate(() => {
      const store = (window as any).__gameStore
      return {
        inventory: store.state.inventory,
        seen_items: store.state.seen_items,
      }
    })
    expect(state.inventory.some((it: any) => it.id === 'oak_logs')).toBe(true)
    expect(state.seen_items.includes('oak_logs')).toBe(true)
  })

  test('re-login preserves discovered items after logout', async ({ page, context }) => {
    // 1. 注册并登录
    const ts = Date.now()
    const username = `persist_${ts}`
    const password = 'test1234'
    const email = `${username}@test.com`

    const res = await context.request.post('http://localhost:8080/api/v1/auth/register', {
      data: { username, password, email },
    })
    expect(res.ok()).toBe(true)

    const login = await context.request.post('http://localhost:8080/api/v1/auth/login', {
      data: { username, password },
    })
    expect(login.ok()).toBe(true)

    await page.goto('/main')
    await page.waitForURL('/main', { timeout: 10000 })

    // 2. 等待 WS 连接和游戏状态
    const wsPromise = page.waitForEvent('websocket', {
      timeout: 15000,
      predicate: (ws) => ws.url().includes('/ws'),
    })
    await wsPromise
    await page.waitForTimeout(2000)

    // 3. 完成起始对话，砍树获得 oak_logs
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
    await page.waitForTimeout(3000)

    await page.evaluate(async () => {
      const actions = (window as any).__actions
      await actions.queueAppend('felling_oak_tree', 3)
      await actions.executeInstant('felling_oak_tree')
    })
    await page.waitForTimeout(6000)

    // 4. 记录当前 oak_logs 数量
    const stateBefore = await page.evaluate(() => {
      const store = (window as any).__gameStore
      return {
        inventory: store.state.inventory,
        seen_items: store.state.seen_items,
      }
    })
    expect(stateBefore.seen_items.includes('oak_logs')).toBe(true)

    // 5. 退出登录（清除 cookie 和 localStorage）
    await page.evaluate(() => {
      localStorage.removeItem('token')
    })
    await context.clearCookies()
    await page.goto('/login')
    await page.waitForTimeout(500)

    // 6. 强制关闭后端 player session（模拟 grace expire）
    const evict = await context.request.post('http://localhost:8080/api/v1/test/evict-session', {
      data: { username },
    })
    expect(evict.ok() || evict.status() === 404).toBe(true)

    // 7. 重新登录（通过 API，像 fixture 一样）
    const login2 = await context.request.post('http://localhost:8080/api/v1/auth/login', {
      data: { username, password },
    })
    expect(login2.ok()).toBe(true)

    await page.goto('/main')
    await page.waitForURL('/main', { timeout: 10000 })

    // 7. 等待 WS 恢复
    const wsPromise2 = page.waitForEvent('websocket', {
      timeout: 15000,
      predicate: (ws) => ws.url().includes('/ws'),
    })
    await wsPromise2
    await page.waitForTimeout(2000)

    // 8. 等待 state.full 被正确应用后再断言
    // （re-login 涉及两次页面导航，比 refresh 需要更长时间）
    await page.waitForFunction(() => {
      const s = (window as any).__gameStore
      return s && Object.keys(s.state.skills).length > 0
    }, { timeout: 15000 })

    const stateAfter = await page.evaluate(() => {
      const store = (window as any).__gameStore
      return {
        inventory: store.state.inventory,
        seen_items: store.state.seen_items,
        unlocked_events: store.state.unlocked_events,
      }
    })
    expect(stateAfter.seen_items.includes('oak_logs')).toBe(true)

    // 9. 切换到制造标签验证事件仍解锁
    const skillRows = page.locator('.skill-row')
    for (let i = 0; i < await skillRows.count(); i++) {
      if ((await skillRows.nth(i).textContent())?.includes('制造')) {
        await skillRows.nth(i).click()
        break
      }
    }
    await page.waitForTimeout(500)

    let found = false
    const cards = page.locator('.event-card')
    for (let i = 0; i < await cards.count(); i++) {
      const title = await cards.nth(i).locator('.event-head strong').textContent()
      if (title?.includes('制作橡木木板')) {
        found = true
        break
      }
    }
    expect(found, '重新登录后仍应能看到制作橡木木板').toBe(true)
  })
})
