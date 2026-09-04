import { expect, test, type Route } from '@playwright/test'
import { stubChatBootstrap } from './helpers/chatBootstrap'

const taskPath = '/api/v1/agent/tasks/TASK-1'
const approvalPath = `${taskPath}/approvals/APPROVAL-1`

const waitingTask = () => ({
  taskId: 'TASK-1', status: 'waiting_approval', revision: 8, persistentStatus: 'waiting_approval',
  workflowProjection: { outcome: 'match', status: 'waiting_approval', revision: 8 },
  pending: {
    kind: 'approval', requestId: 'REQUEST-1', approvalId: 'APPROVAL-1',
    summary: '向项目群发送延期风险提醒', expiresAtUnixMs: Date.now() + 60_000,
  },
})

const runningTask = {
  taskId: 'TASK-1', status: 'running', revision: 9, persistentStatus: 'running',
  workflowProjection: { outcome: 'stale', status: 'running', revision: 9 },
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'approval-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U1', nickname: 'User One' }))
  })
  await page.route('**/api/v1/users/me**', route => ok(route, {
    uuid: 'U1', nickname: 'User One', avatar: '', signature: '', user_type: 0, status: 1,
  }))
  await stubChatBootstrap(page)
})

test('submits the exact approval binding and resumes the Task', async ({ page }) => {
  let submitted: { path: string; authorization: string | undefined; body: unknown } | undefined
  let resolved = false
  await page.route('**/api/v1/agent/tasks/**', async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (request.method() === 'POST' && path === approvalPath) {
      submitted = { path, authorization: request.headers().authorization, body: request.postDataJSON() }
      resolved = true
      await ok(route, null)
      return
    }
    if (request.method() === 'GET' && path === taskPath) {
      await ok(route, resolved ? runningTask : waitingTask())
      return
    }
    await route.fulfill({ status: 404 })
  })

  await page.goto('/app/?agent=1&view=tasks&task=TASK-1&panel=approval')
  await expect(page.getByText('向项目群发送延期风险提醒')).toBeVisible()
  await page.getByRole('button', { name: '批准并继续' }).click()

  await expect(page.getByText('审批已接收，任务继续执行')).toBeVisible()
  expect(submitted).toEqual({
    path: approvalPath,
    authorization: 'Bearer approval-browser-token',
    body: { decision: 'approved' },
  })
})

test('fails closed when the authoritative Task query is unavailable', async ({ page }) => {
  let queries = 0
  await page.route('**/api/v1/agent/tasks/**', async route => {
    queries += 1
    if (queries === 1) {
      await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ message: 'private upstream detail' }) })
      return
    }
    await ok(route, waitingTask())
  })

  await page.goto('/app/?agent=1&view=tasks&task=TASK-1&panel=approval')
  await expect(page.getByRole('alert')).toContainText('无法确认当前审批请求')
  await expect(page.getByText('private upstream detail')).toHaveCount(0)
  await expect(page.getByRole('button', { name: '批准并继续' })).toHaveCount(0)
  await page.getByRole('button', { name: '重新确认' }).click()
  await expect(page.getByRole('button', { name: '批准并继续' })).toBeVisible()
})

test('keeps the approval action surface single-column on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.route('**/api/v1/agent/tasks/**', route => ok(route, waitingTask()))
  await page.goto('/app/?agent=1&view=tasks&task=TASK-1&panel=approval')

  const columns = await page.locator('.approval-grid').evaluate(element => getComputedStyle(element).gridTemplateColumns)
  expect(columns.trim().split(/\s+/)).toHaveLength(1)
  await expect(page.locator('.action-row')).toHaveCSS('flex-direction', 'column-reverse')
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data }),
  })
}
