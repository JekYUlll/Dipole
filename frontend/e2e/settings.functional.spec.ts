import { expect, test, type Route } from '@playwright/test'

const user = { uuid: 'U100', nickname: 'Lin Qiao', telephone: '13800000000', email: 'lin@example.com', signature: 'Build with care', user_type: 0, status: 0, is_admin: false, created_at: '2026-01-01T00:00:00Z' }

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'settings-functional-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Lin Qiao' }))
  })
  await page.route('**/api/v1/users/me', route => ok(route, user))
})

test('loads the authenticated profile without exposing device identifiers', async ({ page }) => {
  await page.goto('/app/settings')
  await expect(page.getByRole('heading', { name: '设置', exact: true })).toBeVisible()
  await expect(page.locator('textarea')).toHaveValue('Build with care')
  await expect(page.locator('.settings-page')).not.toContainText('connection_id')
  await expect(page.getByRole('link', { name: '打开设备安全' })).toHaveAttribute('href', /\/app\/devices$/)
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
