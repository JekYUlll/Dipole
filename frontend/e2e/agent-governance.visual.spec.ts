import { expect, test, type Route } from '@playwright/test'
import { stubChatBootstrap } from './helpers/chatBootstrap'

const subscription = {
  subscriptionId: 'SUB-1', definitionId: 'DEF-1', definitionVersion: 7, agentId: 'UAI',
  eventType: 'message.created', resourceType: 'conversation', resourceId: 'group:G123',
  filterKind: 'message_contains_any', filter: { terms: ['事故', '延期'] }, status: 'active',
  createdById: 'U100', createdAtUnixMs: 1_700_000_000_000, updatedAtUnixMs: 1_700_000_100_000,
}

const memory = {
  memoryId: 'MEM-1', agentId: 'UAI', memoryType: 'semantic', status: 'active',
  resourceType: 'conversation', resourceId: 'group:G1', content: '项目 A 的数据库迁移负责人是 Alice',
  compactContent: 'Owner: Alice', priority: 80,
  provenance: { sourceType: 'message', sourceId: 'MSG-1', sequence: '42' },
  validFromUnixMs: 1_700_000_000_000, createdAtUnixMs: 1_700_000_100_000,
  memoryRootId: 'MEM-1', memoryVersion: 1,
}

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'governance-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
  await stubChatBootstrap(page)
})

test('keeps the subscription governance surface aligned with the Pencil baseline', async ({ page }) => {
  await page.route('**/api/v1/agent/**', async route => {
    const request = route.request()
    const url = new URL(request.url())
    if (request.method() === 'GET' && url.pathname === '/api/v1/agent/subscriptions') {
      await ok(route, { subscriptions: [subscription], nextCursor: '' })
      return
    }
    if (request.method() === 'GET' && url.pathname === '/api/v1/agent/definitions') {
      await ok(route, { definitions: [{
        definitionId: 'DEF-1', version: 7, agentId: 'UAI', conversationScopes: ['group:G123'],
        validFromUnixMs: 1_000, createdAtUnixMs: 1_000, updatedAtUnixMs: 2_000,
      }], nextCursor: '' })
      return
    }
    await route.fulfill({ status: 404 })
  })

  await page.goto('/app/?agent=1&view=subscriptions')
  await expect(page.locator('[data-agent-subscriptions-view]')).toBeVisible()
  await expect(page.locator('[data-agent-subscriptions-view]')).toHaveScreenshot('subscriptions-chromium.png', {
    animations: 'disabled',
  })
})

test('keeps the memory governance surface aligned with the Pencil baseline', async ({ page }) => {
  await page.route('**/api/v1/agent/memory-candidates**', route => ok(route, { candidates: [], nextCursor: '' }))
  await page.route('**/api/v1/agent/memories**', route => ok(route, { memories: [memory], nextCursor: '' }))

  await page.goto('/app/?agent=1&view=memories')
  await expect(page.locator('[data-agent-memories-view]')).toBeVisible()
  await expect(page.locator('[data-agent-memories-view]')).toHaveScreenshot('memories-chromium.png', {
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
