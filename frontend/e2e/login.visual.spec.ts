import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
})

test('keeps the Login desktop surface aligned with the V3 baseline', async ({ page }) => {
  await page.goto('/app/login')
  await expect(page.locator('form').getByRole('button', { name: '登录', exact: true })).toBeVisible()
  await expect(page.locator('.login-page')).toHaveScreenshot('login-desktop-chromium.png', { animations: 'disabled' })
})

test('keeps the Login mobile surface aligned with the V3 baseline', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/app/login')
  await expect(page.locator('form').getByRole('button', { name: '登录', exact: true })).toBeVisible()
  await expect(page.locator('.login-page')).toHaveScreenshot('login-mobile-chromium.png', { animations: 'disabled' })
})
