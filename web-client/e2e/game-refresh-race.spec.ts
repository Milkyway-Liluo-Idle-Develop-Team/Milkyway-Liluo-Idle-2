import { test, expect } from './fixtures'

/**
 * 测试频繁刷新页面时 WS 初始化是否可能漏数据。
 *
 * 已知风险场景：
 * 1. 页面刷新后，router.beforeEach 会提前建立 WS 连接，
 *    而后端会立即推送 state.full。如果此时 Vue 组件还未挂载，
 *    initWsListeners() 尚未执行，state.full 消息会因没有 handler
 *    而被静默丢弃，导致前端所有状态（等级、物品栏等）都显示为空。
 * 2. 即使 initWsListeners 已注册，如果 idRegistry（静态配置）尚未
 *    从 /api/v1/game/config 加载完成，applyStateFull 会直接 return，
 *    导致 state.full 被忽略。
 * 3. 重连时 SendFullState 发送的是数据库快照，而 tick 产生的 diff
 *    可能尚未 flush，导致客户端收到的全量状态落后于实际内存状态。
 *
 * 该测试通过多次快速刷新并断言关键状态字段的完整性来捕捉上述问题。
 */
test.describe('Frequent refresh + WS init data race', () => {
  test.setTimeout(120000)

  test('state remains intact after multiple rapid refreshes', async ({ authPage }) => {
    const { page, ws } = authPage

    // ------------------------------------------------------------------
    // 1. 先执行一些操作，让后端产生非空状态（inventory、skill xp、queue）
    // ------------------------------------------------------------------
    await page.evaluate(async () => {
      const actions = (window as any).__actions
      // 完成起始对话，解锁砍伐
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

    // 砍树并加入队列，使 session 有 inventory + queue
    await page.evaluate(async () => {
      const actions = (window as any).__actions
      await actions.queueAppend('felling_oak_tree', 5)
      await actions.executeInstant('felling_oak_tree')
    })
    // 等待后端 tick 结算，确保状态写入 session
    await page.waitForTimeout(6000)

    // 记录刷新前的核心状态快照
    const snapshotBefore = await page.evaluate(() => {
      const store = (window as any).__gameStore
      return {
        inventory: store.state.inventory as Array<{ id: string; qty: number }>,
        skills: { ...store.state.skills } as Record<string, { level: number; exp: number }>,
        queueItems: [...store.state.queue_items] as Array<{ event_id: string; iterations: number | null }>,
        seenItems: [...store.state.seen_items] as string[],
        unlockedEvents: [...store.state.unlocked_events] as string[],
      }
    })

    expect(snapshotBefore.inventory.length, '刷新前应有 inventory 数据').toBeGreaterThan(0)
    expect(Object.keys(snapshotBefore.skills).length, '刷新前应有 skill 数据').toBeGreaterThan(0)
    expect(snapshotBefore.seenItems.length, '刷新前应有 seen_items').toBeGreaterThan(0)

    // ------------------------------------------------------------------
    // 2. 多次快速刷新页面（不强制 evict session，模拟真实频繁刷新）
    // ------------------------------------------------------------------
    const REFRESH_COUNT = 8

    for (let i = 0; i < REFRESH_COUNT; i++) {
      // 收集本周期内所有 WS 消息，用于事后诊断
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

      // 刷新页面
      await page.reload()
      await page.waitForURL('/main', { timeout: 10000 })

      // 等待 WS 连接重建（必须监听新的 WebSocket 对象）
      const newWs = await page.waitForEvent('websocket', {
        timeout: 15000,
        predicate: (w) => w.url().includes('/ws'),
      })
      newWs.on('framereceived', handler)

      // 等待前端状态稳定：skills 有数据且 inventory 不为空
      await page.waitForFunction(() => {
        const s = (window as any).__gameStore
        return (
          s &&
          Object.keys(s.state.skills).length > 0 &&
          s.state.inventory.length > 0 &&
          s.state.seen_items.length > 0
        )
      }, { timeout: 15000 })

      newWs.off('framereceived', handler)

      // ----------------------------------------------------------------
      // 3. 每次刷新后断言关键状态字段仍然完整
      // ----------------------------------------------------------------
      const snapshotAfter = await page.evaluate(() => {
        const store = (window as any).__gameStore
        return {
          inventory: store.state.inventory as Array<{ id: string; qty: number }>,
          skills: { ...store.state.skills } as Record<string, { level: number; exp: number }>,
          queueItems: [...store.state.queue_items] as Array<{ event_id: string; iterations: number | null }>,
          seenItems: [...store.state.seen_items] as string[],
          unlockedEvents: [...store.state.unlocked_events] as string[],
        }
      })

      // inventory 不能变空
      expect(
        snapshotAfter.inventory.length,
        `第 ${i + 1} 次刷新后 inventory 不应为空`,
      ).toBeGreaterThan(0)

      // skills 不能变空（如果 state.full 丢失，这里会表现为 skills 只剩空对象）
      expect(
        Object.keys(snapshotAfter.skills).length,
        `第 ${i + 1} 次刷新后 skills 不应为空`,
      ).toBeGreaterThan(0)

      // seen_items 不能变空
      expect(
        snapshotAfter.seenItems.length,
        `第 ${i + 1} 次刷新后 seen_items 不应为空`,
      ).toBeGreaterThan(0)

      // 具体物品不应丢失（允许后端继续 tick 导致数量变化，但不允许减少或丢失）
      for (const beforeItem of snapshotBefore.inventory) {
        const afterItem = snapshotAfter.inventory.find(
          (it: any) => it.id === beforeItem.id,
        )
        expect(
          afterItem !== undefined,
          `第 ${i + 1} 次刷新后 inventory 中应保留物品 ${beforeItem.id}`,
        ).toBe(true)
        if (afterItem) {
          // felling 只会增加 oak_logs，不应减少
          expect(
            afterItem.qty >= beforeItem.qty,
            `第 ${i + 1} 次刷新后 ${beforeItem.id} 数量不应减少 (${afterItem.qty} < ${beforeItem.qty})`,
          ).toBe(true)
        }
      }

      // 技能等级不应回退
      for (const [skillId, beforeSkill] of Object.entries(snapshotBefore.skills)) {
        const afterSkill = snapshotAfter.skills[skillId]
        expect(
          afterSkill !== undefined,
          `第 ${i + 1} 次刷新后 skill ${skillId} 不应丢失`,
        ).toBe(true)
        if (afterSkill) {
          expect(
            afterSkill.level >= beforeSkill.level && afterSkill.exp >= beforeSkill.exp,
            `第 ${i + 1} 次刷新后 skill ${skillId} 不应回退`,
          ).toBe(true)
        }
      }

      // unlocked_events 应包含刷新前已解锁的事件
      for (const evtId of snapshotBefore.unlockedEvents) {
        expect(
          snapshotAfter.unlockedEvents.includes(evtId),
          `第 ${i + 1} 次刷新后已解锁事件 ${evtId} 应保持解锁`,
        ).toBe(true)
      }

      // 调试输出
      const stateMsgs = messages.filter(
        (m) => m.type === 'state.diff' || m.type === 'state.full',
      )
      console.log(
        `Refresh ${i + 1}/${REFRESH_COUNT}: WS msgs=${messages.length}, state msgs=${stateMsgs.length}, ` +
        `inventory=${snapshotAfter.inventory.length}, skills=${Object.keys(snapshotAfter.skills).length}`,
      )

      // 每次刷新间隔随机 100~400ms，模拟真实用户快速 F5
      await page.waitForTimeout(100 + Math.floor(Math.random() * 300))
    }
  })

  test('state.full must arrive before any state.diff is applied after refresh', async ({ authPage }) => {
    const { page } = authPage

    // 1. 先建立一些状态
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
      await actions.queueAppend('felling_oak_tree', 5)
      await actions.executeInstant('felling_oak_tree')
    })
    await page.waitForTimeout(6000)

    // 2. 刷新并严格检查消息时序
    await page.reload()
    await page.waitForURL('/main', { timeout: 10000 })

    const newWs = await page.waitForEvent('websocket', {
      timeout: 15000,
      predicate: (w) => w.url().includes('/ws'),
    })

    const messages: Array<{ type: string; idx: number }> = []
    let idx = 0
    const handler = (f: any) => {
      try {
        const msg = JSON.parse(f.payload as string)
        messages.push({ type: msg.type, idx: idx++ })
      } catch {
        messages.push({ type: 'binary/unknown', idx: idx++ })
      }
    }
    newWs.on('framereceived', handler)

    // 等待状态稳定
    await page.waitForFunction(() => {
      const s = (window as any).__gameStore
      return s && Object.keys(s.state.skills).length > 0 && s.state.inventory.length > 0
    }, { timeout: 15000 })

    // 给后端一点时间推送可能产生的 diff
    await page.waitForTimeout(3000)
    newWs.off('framereceived', handler)

    // 3. 断言：state.full 必须是第一条 state 消息（或至少要在第一条 diff 之前）
    const stateMsgIndices = messages
      .map((m, i) => ({ ...m, i }))
      .filter((m) => m.type === 'state.full' || m.type === 'state.diff')

    console.log('State message order:', stateMsgIndices.map((m) => `${m.type}@${m.i}`))

    const firstFullIdx = stateMsgIndices.find((m) => m.type === 'state.full')
    const firstDiffIdx = stateMsgIndices.find((m) => m.type === 'state.diff')

    // 如果没有收到任何 state.full，说明全量状态推送丢失（严重 bug）
    expect(firstFullIdx, '刷新后应收到至少一条 state.full').toBeTruthy()

    // 如果收到了 diff，它必须在第一条 full 之后（先 full 再 diff 才是正确时序）
    if (firstFullIdx && firstDiffIdx) {
      expect(
        firstFullIdx.i <= firstDiffIdx.i,
        'state.full 应在第一条 state.diff 之前到达，否则客户端可能因缺少基准状态而错误应用 diff',
      ).toBe(true)
    }
  })

  /**
   * 该测试专门检测 state.full 在页面刷新后"完全丢失"的场景：
   * 前端路由守卫会提前建立 WS，而后端立即推送 state.full。
   * 如果此时 Vue 组件尚未挂载（initWsListeners 未执行），
   * state.full 会被静默丢弃，导致页面上的等级、物品栏等全部显示为空。
   *
   * 我们通过在页面加载早期（不等 waitForFunction 超时）多次轮询来捕捉
   * 这种"完全空白"的中间状态。
   */
  test('rapid refresh should not cause totally blank state', async ({ authPage }) => {
    const { page, ws } = authPage

    // 1. 建立非空状态
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
      await actions.queueAppend('felling_oak_tree', 5)
      await actions.executeInstant('felling_oak_tree')
    })
    await page.waitForTimeout(6000)

    // 2. 连续刷新并在每次刷新后立即检查是否出现"完全空白"状态
    const REFRESH_COUNT = 5
    for (let i = 0; i < REFRESH_COUNT; i++) {
      const wsMessages: string[] = []
      const msgHandler = (f: any) => {
        try {
          const msg = JSON.parse(f.payload as string)
          wsMessages.push(msg.type)
        } catch {}
      }

      await page.reload()
      await page.waitForURL('/main', { timeout: 10000 })

      // 等待新 WS 连接并监听新对象
      const newWs = await page.waitForEvent('websocket', {
        timeout: 15000,
        predicate: (w) => w.url().includes('/ws'),
      })
      newWs.on('framereceived', msgHandler)

      // 关键：不等 waitForFunction 长超时，而是快速轮询检查
      // 如果在任何时候检测到 skills 为空但 store 已存在，说明 state.full 丢失
      let blankDetected = false
      let stateFullReceived = false
      for (let poll = 0; poll < 30; poll++) {
        const check = await page.evaluate(() => {
          const store = (window as any).__gameStore
          if (!store) return { storeExists: false, skillsEmpty: true, invEmpty: true, idRegistryExists: false, listenersReady: false }
          return {
            storeExists: true,
            skillsEmpty: Object.keys(store.state.skills).length === 0,
            invEmpty: store.state.inventory.length === 0,
            idRegistryExists: !!store.idRegistry,
            listenersReady: !!(store.initWsListeners && store.disposeWsListeners),
          }
        })

        if (check.storeExists && check.skillsEmpty && check.invEmpty) {
          blankDetected = true
          console.log(
            `  [poll ${poll}] blank detected: idRegistry=${check.idRegistryExists}, listenersReady=${check.listenersReady}`,
          )
        }
        if (!check.skillsEmpty && !check.invEmpty) {
          // 状态已经恢复，说明 state.full 最终到达了
          break
        }
        await page.waitForTimeout(100)
      }

      // 检查 Playwright 层面是否收到 state.full
      stateFullReceived = wsMessages.includes('state.full')

      newWs.off('framereceived', msgHandler)

      console.log(
        `Blank-check ${i + 1}/${REFRESH_COUNT}: ` +
        `blankDetected=${blankDetected}, stateFullReceived=${stateFullReceived}, ` +
        `messages=[${wsMessages.join(', ')}]`,
      )

      // 如果出现了完全空白，说明 state.full 在 initWsListeners 之前到达并被丢弃
      // 这是需要修复的前端/后端 race condition
      expect(
        blankDetected,
        `第 ${i + 1} 次刷新后不应出现完全空白状态（skills 和 inventory 同时为空）`,
      ).toBe(false)

      // 同时断言我们确实收到了 state.full（如果后端没发，那是另一个 bug）
      expect(
        stateFullReceived,
        `第 ${i + 1} 次刷新后应收到 state.full 消息`,
      ).toBe(true)
    }
  })
})
