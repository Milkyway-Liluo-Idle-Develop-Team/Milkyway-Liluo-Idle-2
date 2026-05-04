import { test, expect } from './fixtures'

test.describe('Auth pages', () => {
  test('login page loads', async ({ page }) => {
    await page.goto('/login')
    await expect(page.locator('h1')).toContainText('登录')
  })

  test('register page loads', async ({ page }) => {
    await page.goto('/register')
    await expect(page.locator('h1')).toContainText('注册')
  })

  test('login with invalid credentials shows error', async ({ page }) => {
    await page.goto('/login')
    await page.fill('input[aria-label="用户名"]', 'nonexistent_user')
    await page.fill('input[aria-label="密码"]', 'wrongpassword')
    await page.click('button[type="submit"]')

    // Error toast or inline message should appear
    await expect(page.locator('body')).toContainText('invalid credentials', { timeout: 5000 })
  })

  test('authenticated user is redirected from login to main', async ({ page }) => {
    // This test assumes the user is already logged in (cookie present).
    // In a real suite you would log in via API first.
    test.skip(true, 'requires pre-authenticated session')
    await page.goto('/login')
    await page.waitForURL('/main', { timeout: 5000 })
    await expect(page.locator('body')).toContainText('主界面')
  })

  test('unauthenticated user is blocked from main', async ({ page, context }) => {
    // Clear any existing cookies to ensure unauthenticated state
    await context.clearCookies()
    await page.goto('/main')
    await page.waitForURL('/login**', { timeout: 5000 })
  })
})
