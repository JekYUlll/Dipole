import { expect, test, type Route } from '@playwright/test'

const listPath = '/api/v1/agent/memories'
const revokePath = `${listPath}/MEM-1/revoke`
const correctionPath = `${listPath}/MEM-1/correct`
const active = {
  memoryId: 'MEM-1', agentId: 'UAI', memoryType: 'semantic', status: 'active',
  resourceType: 'conversation', resourceId: 'group:G1', content: '项目 A 的数据库迁移负责人是 Alice', compactContent: 'Owner: Alice', priority: 80,
  provenance: { sourceType: 'message', sourceId: 'MSG-1', sequence: '42' },
  validFromUnixMs: 1_700_000_000_000, createdAtUnixMs: 1_700_000_100_000,
  memoryRootId: 'MEM-1', memoryVersion: 1,
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'memory-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
})

test('lists owner Memories and revokes with audited confirmation', async ({ page }) => {
  let submitted: unknown
  await page.route('**/api/v1/agent/memories**', async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (request.method() === 'GET' && path === listPath) {
      await ok(route, { memories: [active], nextCursor: '' })
      return
    }
    if (request.method() === 'POST' && path === revokePath) {
      submitted = { authorization: request.headers().authorization, body: request.postDataJSON() }
      await ok(route, { ...active, status: 'revoked', revokedAtUnixMs: 1_700_000_200_000, revokedById: 'U100', revokeReason: '信息已经过时' })
      return
    }
    await route.fulfill({ status: 404 })
  })

  await page.goto('/app/agent/memories')
  await expect(page.getByRole('heading', { name: '长期记忆' })).toBeVisible()
  await expect(page.getByText('UNTRUSTED MEMORY')).toBeVisible()
  await expect(page.getByText('MSG-1')).toBeVisible()
  await page.locator('[data-agent-memory-revoke="MEM-1"]').click()
  await expect(page.locator('[data-agent-memory-reason]')).toBeFocused()
  await page.locator('[data-agent-memory-reason]').fill('信息已经过时')
  await page.locator('[data-agent-memory-confirm]').click()
  await expect(page.getByText('REVOKED')).toBeVisible()
  expect(submitted).toEqual({ authorization: 'Bearer memory-browser-token', body: { reason: '信息已经过时' } })
})

test('appends a versioned owner correction from the authoritative pair', async ({ page }) => {
  let submitted: unknown
  await page.route('**/api/v1/agent/memories**', async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (request.method() === 'GET' && path === listPath) {
      await ok(route, { memories: [active], nextCursor: '' })
      return
    }
    if (request.method() === 'POST' && path === correctionPath) {
      submitted = request.postDataJSON()
      await ok(route, {
        previous: { ...active, status: 'revoked', revokedAtUnixMs: 1_700_000_200_000, revokedById: 'U100', revokeReason: 'superseded by MEM-2' },
        corrected: {
          ...active, memoryId: 'MEM-2', memoryVersion: 2, memoryRootId: 'MEM-1', supersedesMemoryId: 'MEM-1',
          correctedById: 'U100', correctionReason: '负责人信息已确认', content: '项目 A 的数据库迁移负责人是 Bob', compactContent: 'Owner: Bob',
          provenance: { sourceType: 'owner_correction', sourceId: 'MEM-1', sequence: '2' },
        },
      })
      return
    }
    await route.fulfill({ status: 404 })
  })

  await page.goto('/app/agent/memories')
  await page.locator('[data-agent-memory-correct="MEM-1"]').click()
  await page.locator('[data-agent-memory-correction-content]').fill('项目 A 的数据库迁移负责人是 Bob')
  await page.locator('[data-agent-memory-correction-compact]').fill('Owner: Bob')
  await page.locator('[data-agent-memory-correction-reason]').fill('负责人信息已确认')
  await page.locator('[data-agent-memory-correction-confirm]').click()
  await expect(page.getByText('V2')).toBeVisible()
  await expect(page.getByText('REVOKED')).toBeVisible()
  expect(submitted).toEqual({
    expectedVersion: 1, content: '项目 A 的数据库迁移负责人是 Bob', compactContent: 'Owner: Bob', reason: '负责人信息已确认',
  })
})

test('fails closed and preserves the mobile revoke sheet', async ({ page }) => {
  let calls = 0
  await page.route('**/api/v1/agent/memories**', async route => {
    calls += 1
    if (calls === 1) {
      await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ message: 'private upstream detail' }) })
      return
    }
    await ok(route, { memories: [active], nextCursor: '' })
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/app/agent/memories')
  await expect(page.getByRole('alert')).toContainText('记忆控制面暂时不可用')
  await expect(page.getByText('private upstream detail')).toHaveCount(0)
  await page.locator('[data-agent-memory-retry]').click()
  await page.locator('[data-agent-memory-revoke="MEM-1"]').click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.locator('.control-rail')).toBeHidden()
  await expect(page.locator('.revoke-dialog')).toHaveCSS('width', '390px')
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
