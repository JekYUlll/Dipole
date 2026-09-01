import { expect, test, type Route } from '@playwright/test'

const sessions = [{ connection_id: 'C100', device: 'Chrome on Linux', device_id: 'D100', user_agent: 'Chrome 140', remote_addr: '10.0.0.1', node_id: 'N1', connected_at: '2026-09-01T10:00:00Z', last_seen_at: '2026-09-01T10:01:00Z' }]

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'device-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Lin Qiao' }))
  })
  await page.route('**/api/v1/users/me/devices**', route => ok(route, sessions))
})

test('keeps the device directory aligned with the V3 desktop baseline', async ({ page }) => {
  await page.goto('/app/devices')
  await expect(page.getByRole('heading', { name: '设备会话', exact: true })).toBeVisible()
  await expect(page.locator('.device-directory')).toContainText('Chrome on Linux')
  await expect(page.locator('.device-directory')).toHaveScreenshot('devices-desktop-chromium.png', { animations: 'disabled' })
})

test('keeps the device directory aligned with the V3 mobile baseline', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/app/devices')
  await expect(page.getByRole('heading', { name: '设备会话', exact: true })).toBeVisible()
  await expect(page.locator('.device-directory')).toHaveScreenshot('devices-mobile-chromium.png', { animations: 'disabled' })
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
