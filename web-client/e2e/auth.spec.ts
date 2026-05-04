import { test, expect } from './fixtures'

test.describe('Auth flow', () => {
  test('register → auto login → redirect to /main', async ({ page }) => {
    const username = `reg_${Date.now()}`

    await page.goto('/register')
    await expect(page.locator('h1')).toContainText('注册')

    // Use #register-form scope to avoid collision with login form inputs
    await page.fill('#register-form input[aria-label="用户名"]', username)
    await page.fill('#register-form input[aria-label="邮箱"]', `${username}@t.com`)
    await page.fill('#register-form input[aria-label="密码"]', 'testpass123')
    await page.fill('#register-form input[aria-label="确认密码"]', 'testpass123')
    await page.click('button:has-text("注册")')

    // Should redirect to /main after successful registration
    await page.waitForURL('/main', { timeout: 10000 })
    await expect(page.locator('body')).toBeVisible()

    // Cleanup
    await page.request.post('http://localhost:8080/api/v1/test/delete-user', {
      data: { username },
    })
  })

  test('login with bad credentials shows error', async ({ page }) => {
    await page.goto('/login')
    await page.fill('#login-form input[aria-label="用户名"]', 'nobody_' + Date.now())
    await page.fill('#login-form input[aria-label="密码"]', 'wrongpass')
    await page.click('button:has-text("登录")')

    await expect(page.locator('body')).toContainText('invalid credentials', { timeout: 5000 })
  })

  test('unauthenticated user is blocked from /main', async ({ page, context }) => {
    await context.clearCookies()
    await page.goto('/main')
    await page.waitForURL('/login**', { timeout: 5000 })
  })

  test('logout redirects to login', async ({ authPage, context }) => {
    const { page } = authPage

    // Call logout API then clear cookies
    await page.request.post('http://localhost:8080/api/v1/auth/logout')
    await context.clearCookies()

    await page.reload()
    await page.waitForURL('/login**', { timeout: 5000 })
  })
})
