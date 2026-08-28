import { expect, test, type Route } from '@playwright/test'

const listPath = '/api/v1/agent/subscriptions'
const revokePath = `${listPath}/SUB-1/revoke`
const active = {
  subscriptionId: 'SUB-1', definitionId: 'DEF-1', definitionVersion: 7, agentId: 'UAI',
  eventType: 'message.created', resourceType: 'conversation', resourceId: 'group:G123',
  filterKind: 'message_contains_any', filter: { terms: ['事故', '延期'] }, status: 'active',
  createdById: 'U100', createdAtUnixMs: 1_000, updatedAtUnixMs: 2_000,
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'subscription-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
})

test('lists owner subscriptions and revokes with an exact audited reason', async ({ page }) => {
  let submitted: { path: string; authorization?: string; body: unknown } | undefined
  await page.route('**/api/v1/agent/subscriptions**', async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (request.method() === 'GET' && path === listPath) {
      await ok(route, { subscriptions: [active], nextCursor: '' })
      return
    }
    if (request.method() === 'POST' && path === revokePath) {
      submitted = { path, authorization: request.headers().authorization, body: request.postDataJSON() }
      await ok(route, {
        ...active, status: 'revoked', revokedById: 'U100', revokeReason: '项目已归档',
        revokedAtUnixMs: 3_000, updatedAtUnixMs: 3_000,
      })
      return
    }
    await route.fulfill({ status: 404 })
  })

  await page.goto('/app/agent/subscriptions')
  await expect(page.getByRole('heading', { name: '事件订阅' })).toBeVisible()
  await expect(page.getByText('DIRECT_TARGET').first()).toBeVisible()
  await expect(page.locator('[data-agent-subscription-create]')).toBeDisabled()
  await page.locator('[data-agent-subscription-revoke="SUB-1"]').click()
  await expect(page.locator('[data-agent-subscription-reason]')).toBeFocused()
  await page.locator('[data-agent-subscription-reason]').fill('项目已归档')
  await page.locator('[data-agent-subscription-confirm]').click()
  await expect(page.getByText('REVOKED')).toBeVisible()
  expect(submitted).toEqual({
    path: revokePath, authorization: 'Bearer subscription-browser-token', body: { reason: '项目已归档' },
  })
})

test('fails closed on unavailable authority and preserves the mobile revoke sheet', async ({ page }) => {
  let calls = 0
  await page.route('**/api/v1/agent/subscriptions**', async route => {
    calls += 1
    if (calls === 1) {
      await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ message: 'private upstream detail' }) })
      return
    }
    await ok(route, { subscriptions: [active], nextCursor: '' })
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/app/agent/subscriptions')
  await expect(page.getByRole('alert')).toContainText('订阅控制面暂时不可用')
  await expect(page.getByText('private upstream detail')).toHaveCount(0)
  await expect(page.locator('[data-agent-subscription-revoke]')).toHaveCount(0)
  await page.locator('[data-agent-subscription-retry]').click()
  await page.locator('[data-agent-subscription-revoke="SUB-1"]').click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.locator('.control-rail')).toBeHidden()
  await expect(page.locator('.revoke-dialog')).toHaveCSS('width', '390px')
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
