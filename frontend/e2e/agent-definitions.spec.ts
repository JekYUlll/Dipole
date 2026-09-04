import { expect, test, type Route } from '@playwright/test'
import { stubChatBootstrap } from './helpers/chatBootstrap'

const catalog = {
  definitions: [{
    definitionId: 'DEF-READ-1', version: 7, agentId: 'UAI', conversationScopes: ['group:G123', 'direct:U100:U200'],
    validFromUnixMs: 1_700_000_000_000, createdAtUnixMs: 1_700_000_000_000, updatedAtUnixMs: 1_700_000_060_000,
  }],
  nextCursor: '',
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'definition-catalog-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
  await stubChatBootstrap(page)
})

test('renders the authenticated read-only Agent Definition catalog', async ({ page }) => {
  await page.route('**/api/v1/agent/definitions**', route => ok(route, catalog))

  await page.goto('/app/?agent=1&view=definitions')
  await expect(page.locator('[data-agent-definitions-view]')).toBeVisible()
  await expect(page.locator('[data-agent-definition-id="DEF-READ-1"]')).toBeVisible()
  await expect(page.getByText('group:G123')).toBeVisible()
  await expect(page.locator('.state-card')).toHaveCount(0)
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data }),
  })
}
