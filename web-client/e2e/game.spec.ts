import { test, expect } from './fixtures'

test.describe('Game session + WebSocket', () => {
  test('WS connects and state is populated', async ({ authPage }) => {
    const { page, ws } = authPage
    expect(ws.url()).toContain('ws://')

    const hasSkills = await page.evaluate(() => {
      const store = (window as any).__gameStore
      return store && Object.keys(store.state.skills).length > 0
    })
    expect(hasSkills).toBe(true)
  })

  test('execute instant event without error', async ({ authPage }) => {
    const { page, ws } = authPage

    // Collect WS messages for 3s
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

    const result = await page.evaluate(() => {
      const store = (window as any).__gameStore
      const actions = (window as any).__actions

      const events = store.loopEvents as Array<{ id: string; name: string }>
      const target = events[0]
      if (!target) return { ok: false, reason: 'no events in config', eventsCount: events.length }

      try {
        actions.executeInstant(target.id)
        return { ok: true, eventId: target.id }
      } catch (e: any) {
        return { ok: false, reason: e?.message || String(e), eventId: target.id }
      }
    })

    expect(result.ok).toBe(true)

    await page.waitForTimeout(3000)
    ws.off('framereceived', handler)

    console.log('WS messages (instant):', messages.map((m) => m.type))
    const stateMessages = messages.filter((m) => m.type === 'state.diff' || m.type === 'state.full')
    expect(stateMessages.length).toBeGreaterThan(0)
  })

  test('queue append without error', async ({ authPage }) => {
    const { page, ws } = authPage

    const messages: Array<{ type: string }> = []
    const handler = (f: any) => {
      try {
        const msg = JSON.parse(f.payload as string)
        messages.push({ type: msg.type })
      } catch {}
    }
    ws.on('framereceived', handler)

    const result = await page.evaluate(() => {
      const store = (window as any).__gameStore
      const actions = (window as any).__actions

      const events = store.loopEvents as Array<{ id: string; name: string }>
      const target = events[0]
      if (!target) return { ok: false, reason: 'no events in config' }

      try {
        actions.queueAppend(target.id, 5)
        return { ok: true, eventId: target.id }
      } catch (e: any) {
        return { ok: false, reason: e?.message || String(e) }
      }
    })

    expect(result.ok).toBe(true)

    await page.waitForTimeout(3000)
    ws.off('framereceived', handler)

    console.log('WS messages (queue):', messages.map((m) => m.type))
    const stateMessages = messages.filter((m) => m.type === 'state.diff' || m.type === 'state.full')
    expect(stateMessages.length).toBeGreaterThan(0)
  })
})
