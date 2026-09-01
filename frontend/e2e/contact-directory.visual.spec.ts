import { expect, test, type Route } from '@playwright/test'

const contact = {
  user: { uuid: 'U200', nickname: 'Lin Qiao', avatar: '', signature: 'Timeline review', user_type: 0, status: 0 },
  remark: 'Migration partner',
  status: 0,
}

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'contact-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'User One' }))
  })
  await page.route('**/api/v1/contacts', route => ok(route, [contact]))
})

test('keeps the Contact directory aligned with the V3 desktop baseline', async ({ page }) => {
  await page.goto('/app/contacts')
  await expect(page.getByRole('heading', { name: '联系人', exact: true })).toBeVisible()
  await expect(page.locator('.contact-directory')).toHaveScreenshot('contacts-desktop-chromium.png', { animations: 'disabled' })
})

test('keeps the Contact directory aligned with the V3 mobile baseline', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/app/contacts')
  await expect(page.getByRole('heading', { name: '联系人', exact: true })).toBeVisible()
  await expect(page.locator('.contact-directory')).toHaveScreenshot('contacts-mobile-chromium.png', { animations: 'disabled' })
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
