import { expect, test, type Route } from '@playwright/test'

const page = {
  files: [{ file_id: 'F-HANDOFF-01', file_name: 'platform-handoff.md', file_size: 18_432, content_type: 'text/markdown', created_at: '2026-08-31T08:30:00.000Z', download_path: '/api/v1/files/F-HANDOFF-01/download' }],
  has_more: false,
}

test.beforeEach(async ({ page: browserPage }) => {
  await browserPage.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'file-directory-functional-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
  await browserPage.route(/\/api\/v1\/files(?:\?.*)?$/, route => ok(route, page))
})

test('loads the owner-safe file projection across browsers', async ({ page: browserPage }) => {
  await browserPage.goto('/app/files')
  await expect(browserPage.locator('[data-file-state="ready"]')).toBeVisible()
  await expect(browserPage.getByText('platform-handoff.md')).toBeVisible()
  await expect(browserPage.locator('[data-file-state="ready"]')).not.toContainText('object_key')
  await expect(browserPage.locator('[data-file-state="ready"]')).not.toContainText('storage URL')
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
