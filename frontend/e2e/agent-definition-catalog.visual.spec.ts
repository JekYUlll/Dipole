import { expect, test, type Route } from '@playwright/test'

const definition = {
  definitionId: 'DEF-1', version: 7, agentId: 'UAI', conversationScopes: ['group:G123', 'direct:U100:UAI'],
  validFromUnixMs: 1_700_000_000_000, createdAtUnixMs: 1_700_000_000_000, updatedAtUnixMs: 1_700_000_100_000,
}

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'definition-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'User One' }))
  })
  await page.route('**/api/v1/agent/definitions?**', route => ok(route, { definitions: [definition], nextCursor: '' }))
})

test('keeps the Definition catalog aligned with the V3 desktop baseline', async ({ page }) => {
  await page.goto('/app/agent/definitions')
  await expect(page.getByRole('heading', { name: 'Agent 定义' })).toBeVisible()
  await expect(page.locator('.definition-shell')).toHaveScreenshot('definitions-desktop-chromium.png', { animations: 'disabled' })
})

test('keeps the Definition catalog aligned with the V3 mobile baseline', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/app/agent/definitions')
  await expect(page.locator('.definition-shell')).toHaveScreenshot('definitions-mobile-chromium.png', { animations: 'disabled' })
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
