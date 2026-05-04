import { test, expect } from './fixtures'
import * as fs from 'fs'
import * as path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const shotDir = path.join(__dirname, '..', 'test-results', 'walkthrough')
if (!fs.existsSync(shotDir)) fs.mkdirSync(shotDir, { recursive: true })

async function shot(page: any, name: string) {
  const p = path.join(shotDir, `${name}.png`)
  await page.screenshot({ path: p, fullPage: true })
  console.log(`📸 ${p}`)
  return p
}

test.describe('Interactive walkthrough', () => {
  test.setTimeout(180000)

  test('play through starting dialogs, chop wood, queue multiple events', async ({ authPage }) => {
    const { page } = authPage
    let step = 0
    const next = async (label: string) => {
      step++
      await shot(page, `${String(step).padStart(2, '0')}-${label}`)
    }

    // ── 1. 初始页面 ──
    await next('initial-main')

    // ── 2. 快速完成所有对话解锁 ──
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
    await page.waitForTimeout(5000)

    // ── 3. 切换到砍伐标签 ──
    const skillRows = page.locator('.skill-row')
    for (let i = 0; i < await skillRows.count(); i++) {
      if ((await skillRows.nth(i).textContent())?.includes('砍伐')) {
        await skillRows.nth(i).click()
        break
      }
    }
    await page.waitForTimeout(500)
    await next('felling-tab')

    // ── 4. 展开队列面板（如果折叠） ──
    const queueToggle = page.locator('.queue-toggle')
    if (await queueToggle.isVisible().catch(() => false)) {
      await queueToggle.click()
      await page.waitForTimeout(200)
    }

    // ── 5. 找到砍伐橡木事件，输入 5 次，点击 +队列 ──
    const eventCards = page.locator('.event-card')
    let chopCard: import('@playwright/test').Locator | null = null
    for (let i = 0; i < await eventCards.count(); i++) {
      const card = eventCards.nth(i)
      if ((await card.locator('.event-head strong').textContent())?.includes('砍伐橡木')) {
        chopCard = card
        break
      }
    }
    if (!chopCard) {
      test.skip(true, '砍伐橡木 not found')
      return
    }

    // 输入 5 次迭代
    const input = chopCard.locator('.iterations-input')
    await input.fill('5')
    await page.waitForTimeout(200)
    await next('before-queue-append')

    // 点击 +队列
    const queueBtn = chopCard.locator('.event-action.secondary')
    await queueBtn.click()
    await page.waitForTimeout(2000)
    await next('after-queue-append-1')

    // ── 6. 再次点击 +队列（再排 5 次） ──
    await input.fill('3')
    await page.waitForTimeout(200)
    await queueBtn.click()
    await page.waitForTimeout(2000)
    await next('after-queue-append-2')

    // ── 7. 点击开始循环（执行队列中的第一个） ──
    const executeBtn = chopCard.locator('.event-action').first()
    await executeBtn.click()
    await page.waitForTimeout(1000)
    await next('queue-started')

    // ── 8. 等待 6 秒，看队列消耗进度 ──
    await page.waitForTimeout(6000)
    await next('queue-after-6s')

    // ── 9. 再次排入一个无限制事件 ──
    await input.fill('')
    await page.waitForTimeout(200)
    await queueBtn.click()
    await page.waitForTimeout(1000)
    await next('after-unlimited-queue')

    // ── 10. 停止循环 ──
    const stopBtn = page.locator('.loop-stop')
    if (await stopBtn.isVisible().catch(() => false)) {
      await stopBtn.click()
      await page.waitForTimeout(1000)
    }
    await next('loop-stopped')

    // ── 11. 切换到制造标签，验证制作橡木木板已解锁 ──
    for (let i = 0; i < await skillRows.count(); i++) {
      if ((await skillRows.nth(i).textContent())?.includes('制造')) {
        await skillRows.nth(i).click()
        break
      }
    }
    await page.waitForTimeout(500)

    const craftCards = page.locator('.event-card')
    let foundPlank = false
    for (let i = 0; i < await craftCards.count(); i++) {
      const title = await craftCards.nth(i).locator('.event-head strong').textContent()
      if (title?.includes('制作橡木木板')) {
        foundPlank = true
        break
      }
    }
    expect(foundPlank, '获得 oak_logs 后应解锁制作橡木木板').toBe(true)
    await next('crafting-unlocked')

    // ── 12. 打印最终状态 ──
    const state = await page.evaluate(() => {
      const store = (window as any).__gameStore
      return {
        inventory: store.state.inventory,
        skills: store.state.skills,
        queue: store.state.queue_items,
      }
    })
    console.log('Final state:', JSON.stringify(state, null, 2))
    await next('final')
  })
})
