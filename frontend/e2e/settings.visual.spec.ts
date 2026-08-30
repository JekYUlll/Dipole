import { expect, test, type Route } from '@playwright/test'

const currentUser = {
  uuid: 'U100',
  nickname: 'Owner',
  avatar: '',
  signature: '让每一次协作都能留下清晰的上下文。',
  user_type: 0,
  status: 0,
  telephone: '',
  email: 'owner@example.test',
  is_admin: false,
  created_at: '2026-08-31T08:30:00.000Z',
}

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'settings-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
})

test('keeps Settings aligned with the Pencil account and disclosure boundary', async ({ page }) => {
  await page.route('**/api/v1/users/me', route => ok(route, currentUser))

  await page.goto('/app/settings')
  await expect(page.getByRole('heading', { name: '设置' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '个人资料', level: 2 })).toBeVisible()
  await expect(page.getByText('打开设备安全')).toBeVisible()
  await expect(page.getByText('安全游标')).toBeVisible()
  await expect(page.getByText('退出当前账户')).toBeVisible()
  await expect(page.locator('.settings-page')).toHaveScreenshot('settings-chromium.png', { animations: 'disabled' })
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
