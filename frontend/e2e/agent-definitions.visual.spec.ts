import { expect, test, type Route } from '@playwright/test'
import { stubChatBootstrap } from './helpers/chatBootstrap'

const catalog = {
  definitions: [{
    definitionId: 'DEF-VISUAL-1', version: 14, agentId: 'UAI', conversationScopes: ['group:G123', 'direct:U100:U200'],
    validFromUnixMs: 1_700_000_000_000, expiresAtUnixMs: 1_900_000_000_000,
    createdAtUnixMs: 1_700_000_000_000, updatedAtUnixMs: 1_700_000_060_000,
  }],
  nextCursor: '',
}

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'definition-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
  await stubChatBootstrap(page)
})

test('keeps the Definition catalog read-only surface aligned with the Pencil baseline', async ({ page }) => {
  await page.route('**/api/v1/agent/definitions**', route => ok(route, catalog))

  await page.goto('/app/?agent=1&view=definitions')
  await expect(page.locator('[data-agent-definition-id="DEF-VISUAL-1"]')).toBeVisible()
  await expect(page.locator('[data-agent-definitions-view]')).toHaveScreenshot('definition-catalog-chromium.png', {
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
