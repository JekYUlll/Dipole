import { expect, test, type Route } from '@playwright/test'
import { stubChatBootstrap } from './helpers/chatBootstrap'

const listPath = '/api/v1/agent/memories'
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
  await stubChatBootstrap(page)
  await page.route('**/api/v1/agent/memory-candidates**', route => ok(route, { candidates: [], nextCursor: '' }))
})

test('lists owner Memories in the drawer without a hero state card', async ({ page }) => {
  await page.route('**/api/v1/agent/memories**', async route => {
    const path = new URL(route.request().url()).pathname
    if (route.request().method() === 'GET' && path === listPath) {
      await ok(route, { memories: [active], nextCursor: '' })
      return
    }
    await route.fulfill({ status: 404 })
  })

  await page.goto('/app/?agent=1&view=memories')
  await expect(page.locator('[data-agent-memories-view]')).toBeVisible()
  await expect(page.getByText('Owner: Alice')).toBeVisible()
  await expect(page.locator('.state-card')).toHaveCount(0)
})

test('appends a versioned owner correction from the authoritative pair', async ({ page }) => {
  let submitted: unknown
  let listed = [active]
  await page.route('**/api/v1/agent/memories**', async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (request.method() === 'GET' && path === listPath) {
      await ok(route, { memories: listed, nextCursor: '' })
      return
    }
    if (request.method() === 'POST' && path === correctionPath) {
      submitted = request.postDataJSON()
      const corrected = {
        ...active, memoryId: 'MEM-2', memoryVersion: 2, memoryRootId: 'MEM-1', supersedesMemoryId: 'MEM-1',
        correctedById: 'U100', correctionReason: '负责人信息已确认', content: '项目 A 的数据库迁移负责人是 Bob', compactContent: 'Owner: Bob',
        provenance: { sourceType: 'owner_correction', sourceId: 'MEM-1', sequence: '2' },
      }
      listed = [corrected]
      await ok(route, {
        previous: { ...active, status: 'revoked', revokedAtUnixMs: 1_700_000_200_000, revokedById: 'U100', revokeReason: 'superseded by MEM-2' },
        corrected,
      })
      return
    }
    await route.fulfill({ status: 404 })
  })

  await page.goto('/app/?agent=1&view=memories')
  await page.locator('[data-agent-memory-correct="MEM-1"]').click()
  await page.locator('[data-agent-memory-correction-content]').fill('项目 A 的数据库迁移负责人是 Bob')
  await page.locator('[data-agent-memory-correction-compact]').fill('Owner: Bob')
  await page.locator('[data-agent-memory-correction-reason]').fill('负责人信息已确认')
  await page.locator('[data-agent-memory-correction-confirm]').click()
  await expect(page.getByText('Owner: Bob')).toBeVisible()
  expect(submitted).toEqual({
    expectedVersion: 1, content: '项目 A 的数据库迁移负责人是 Bob', compactContent: 'Owner: Bob', reason: '负责人信息已确认',
  })
})

test('fails closed with a banner instead of a full-page state card', async ({ page }) => {
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
  await page.goto('/app/?agent=1&view=memories')
  await expect(page.locator('.banner')).toContainText('记忆列表读取失败')
  await expect(page.getByText('private upstream detail')).toHaveCount(0)
  await expect(page.locator('.state-card')).toHaveCount(0)
  await page.getByRole('button', { name: 'Retry' }).click()
  await expect(page.getByText('Owner: Alice')).toBeVisible()
  await expect(page.locator('.control-rail')).toHaveCount(0)
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
