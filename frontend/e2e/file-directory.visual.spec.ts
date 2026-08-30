import { expect, test, type Route } from '@playwright/test'

const page = {
  files: [
    {
      file_id: 'F-HANDOFF-01',
      file_name: 'platform-handoff.md',
      file_size: 18_432,
      content_type: 'text/markdown',
      created_at: '2026-08-31T08:30:00.000Z',
      download_path: '/api/v1/files/F-HANDOFF-01/download',
    },
    {
      file_id: 'F-ARCHIVE-02',
      file_name: 'sync-window-report.pdf',
      file_size: 2_097_152,
      content_type: 'application/pdf',
      created_at: '2026-08-30T16:20:00.000Z',
      download_path: '/api/v1/files/F-ARCHIVE-02/download',
    },
  ],
  next_cursor: '',
  has_more: false,
}

test.beforeEach(async ({ page: browserPage, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await browserPage.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'file-directory-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
})

test('keeps the owner-scoped File Directory aligned with the Pencil disclosure boundary', async ({ page: browserPage }) => {
  await browserPage.route(/\/api\/v1\/files(?:\?.*)?$/, route => ok(route, page))

  await browserPage.goto('/app/files')
  await expect(browserPage.locator('[data-file-state="ready"]')).toBeVisible()
  await expect(browserPage.getByText('platform-handoff.md')).toBeVisible()
  await expect(browserPage.getByText('存储 URL、校验值或未完成上传会话')).toBeVisible()
  await expect(browserPage.locator('[data-file-state="ready"]')).toHaveScreenshot('file-directory-chromium.png', {
    animations: 'disabled',
  })
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data }),
  })
}
