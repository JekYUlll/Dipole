import { expect, test, type Route } from '@playwright/test'

const sessions = [
  { connection_id: 'C-current', device: 'web', device_id: 'device-current', connected_at: '2026-08-30T00:00:00.000Z', last_seen_at: '2026-08-30T00:01:00.000Z' },
  { connection_id: 'C-other', device: 'desktop', device_id: 'device-other', connected_at: '2026-08-30T00:00:00.000Z', last_seen_at: '2026-08-30T00:02:00.000Z' },
]

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'device-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
    localStorage.setItem('dipole.web.deviceID', 'device-current')
  })
})

test('renders the low-sensitive session list and confirms logout-other-devices before acting', async ({ page }) => {
  let logoutOthersCalls = 0
  await page.route('**/api/v1/users/me/devices', route => ok(route, sessions))
  await page.route('**/api/v1/users/me/devices/logout-others', async route => {
    logoutOthersCalls += 1
    await ok(route, { message: 'other device sessions logged out' })
  })
  await page.goto('/app/devices')
  await expect(page.getByRole('heading', { name: '设备会话', exact: true })).toBeVisible()
  await expect(page.getByText('浏览器会话')).toBeVisible()
  await expect(page.getByText('桌面设备')).toBeVisible()
  await expect(page.locator('.device-security')).not.toContainText('C-current')
  await expect(page.locator('.device-security')).not.toContainText('remote_addr')
  await page.getByTestId('logout-others').click()
  await expect(page.getByText('登出所有其他设备？')).toBeVisible()
  expect(logoutOthersCalls).toBe(0)
  await page.getByTestId('confirm-logout').click()
  await expect.poll(() => logoutOthersCalls).toBe(1)
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
