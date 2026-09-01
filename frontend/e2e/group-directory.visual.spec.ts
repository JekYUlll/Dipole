import { expect, test, type Route } from '@playwright/test'

const group = {
  uuid: 'G100', name: 'Atlas Product Guild', notice: '', avatar: '', status: 0,
  member_count: 24, is_hot: true, recent_message_count: 3, me_role: 1,
  owner: { uuid: 'U100', nickname: 'Lin Qiao', avatar: '', signature: 'Project partner', user_type: 0, status: 0 },
  members: [], created_at: '2026-09-01T10:00:00Z',
}

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'group-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Lin Qiao' }))
  })
  await page.route('**/api/v1/conversations?limit=50', route => ok(route, [{
    conversation_key: 'group:G100', target_type: 1, target_group: {}, remark: '',
    last_message: { message_id: 'M100', message_type: 0, preview: 'ship it', sent_at: '2026-09-01T10:00:00Z', sender_uuid: 'U100' },
    unread_count: 1, last_message_seq: 3, read_seq: 2,
  }]))
  await page.route('**/api/v1/groups/G100', route => ok(route, group))
})

test('keeps the Group directory aligned with the V3 desktop baseline', async ({ page }) => {
  await page.goto('/app/groups')
  await expect(page.getByRole('heading', { name: '协作群组', exact: true })).toBeVisible()
  await expect(page.locator('.group-list')).toHaveScreenshot('groups-desktop-chromium.png', { animations: 'disabled' })
})

test('keeps the Group directory aligned with the V3 mobile baseline', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/app/groups')
  await expect(page.getByRole('heading', { name: '协作群组', exact: true })).toBeVisible()
  await expect(page.locator('.group-directory')).toHaveScreenshot('groups-mobile-chromium.png', { animations: 'disabled' })
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
