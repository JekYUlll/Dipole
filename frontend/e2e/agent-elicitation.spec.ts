import { expect, test, type Page, type Route } from '@playwright/test'
import { stubChatBootstrap } from './helpers/chatBootstrap'

const taskPath = '/api/v1/agent/tasks/TASK-1'
const inputPath = `${taskPath}/inputs/INPUT-1`

const waitingTask = () => ({
  taskId: 'TASK-1', status: 'waiting_input', revision: 22, persistentStatus: 'running',
  workflowProjection: { outcome: 'match', status: 'waiting_input', revision: 22 },
  pending: {
    kind: 'input', requestId: 'INPUT-1', prompt: 'Choose event settings', expiresAtUnixMs: Date.now() + 60_000,
    source: { kind: 'mcp', serverId: 'calendar.example', toolName: 'calendar.create', invocationId: 'INV-1', trust: 'untrusted' },
    form: { schemaVersion: 'dipole.agent.elicitation.v1', fields: [
      { id: 'title', label: 'Event title', type: 'text', required: true, maxLength: 120 },
      { id: 'visibility', label: 'Visibility', type: 'select', required: true, options: ['team', 'private'] },
      { id: 'labels', label: 'Labels', type: 'multiselect', required: false, options: ['release', 'incident'], maxSelections: 1 },
      { id: 'notify', label: 'Notify attendees', type: 'boolean', required: false },
    ] },
  },
})

const runningTask = {
  taskId: 'TASK-1', status: 'running', revision: 23, persistentStatus: 'running',
  workflowProjection: { outcome: 'stale', status: 'waiting_input', revision: 22 },
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'elicitation-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U1', nickname: 'User One' }))
  })
  await stubChatBootstrap(page)
})

test('submits the exact Task/request binding and discloses untrusted MCP source', async ({ page }) => {
  let submitted: { path: string; authorization: string | undefined; body: unknown } | undefined
  let completed = false
  await page.route('**/api/v1/agent/tasks/**', async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (request.method() === 'POST' && path === inputPath) {
      submitted = {
        path,
        authorization: request.headers().authorization,
        body: request.postDataJSON(),
      }
      completed = true
      await ok(route, null)
      return
    }
    if (request.method() === 'GET' && path === taskPath) {
      await ok(route, completed ? runningTask : waitingTask())
      return
    }
    await route.fulfill({ status: 404 })
  })

  await page.goto('/app/?agent=1&view=tasks&task=TASK-1&panel=input')
  await expect(page.getByText('UNTRUSTED MCP')).toBeVisible()
  await expect(page.getByText('calendar.example')).toBeVisible()
  await page.getByRole('textbox', { name: 'Event title' }).fill('Release review')
  await page.getByRole('combobox', { name: 'Visibility' }).selectOption('team')
  await page.getByLabel('release').check()
  await page.getByLabel('启用').check()
  await page.getByRole('button', { name: '提交并继续任务' }).click()

  await expect(page.getByText('信息已接收，任务继续执行')).toBeVisible()
  expect(submitted).toEqual({
    path: inputPath,
    authorization: 'Bearer elicitation-browser-token',
    body: { value: { title: 'Release review', visibility: 'team', labels: ['release'], notify: true } },
  })
})

test('focuses the first invalid field and hides stale data while query is unavailable', async ({ page }) => {
  let queries = 0
  await page.route('**/api/v1/agent/tasks/**', async route => {
    queries += 1
    if (queries === 1) {
      await route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ message: 'private upstream detail' }) })
      return
    }
    await ok(route, waitingTask())
  })

  await page.goto('/app/?agent=1&view=tasks&task=TASK-1&panel=input')
  await expect(page.getByRole('alert')).toContainText('无法确认当前输入请求')
  await expect(page.getByText('private upstream detail')).toHaveCount(0)
  await expect(page.locator('[data-agent-submit]')).toHaveCount(0)
  await page.getByRole('button', { name: '重新确认' }).click()
  await page.getByRole('button', { name: '提交并继续任务' }).click()
  await expect(page.getByText('Event title为必填项')).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Event title' })).toBeFocused()
})

test('keeps the maintained single-column mobile layout', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.route('**/api/v1/agent/tasks/**', route => ok(route, waitingTask()))
  await page.goto('/app/?agent=1&view=tasks&task=TASK-1&panel=input')

  const columns = await page.locator('.elicitation-grid').evaluate(element => getComputedStyle(element).gridTemplateColumns)
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
