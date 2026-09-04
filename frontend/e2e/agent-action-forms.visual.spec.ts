import { expect, test, type Route } from '@playwright/test'
import { stubChatBootstrap } from './helpers/chatBootstrap'

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'visual-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U1', nickname: 'User One' }))
  })
  await page.route('**/api/v1/users/me**', route => ok(route, {
    uuid: 'U1', nickname: 'User One', avatar: '', signature: '', user_type: 0, status: 1,
  }))
  await stubChatBootstrap(page)
})

test('keeps the approval surface aligned with the Pencil baseline', async ({ page }) => {
  await page.route('**/api/v1/agent/tasks/TASK-1', route => ok(route, {
    taskId: 'TASK-1', status: 'waiting_approval', revision: 8, persistentStatus: 'waiting_approval',
    workflowProjection: { outcome: 'match', status: 'waiting_approval', revision: 8 },
    pending: {
      kind: 'approval', requestId: 'REQUEST-1', approvalId: 'APPROVAL-1',
      summary: '向项目群发送延期风险提醒', expiresAtUnixMs: Date.now() + 120_000,
    },
  }))

  await page.goto('/app/?agent=1&view=tasks&task=TASK-1&panel=approval')
  await expect(page.getByText('向项目群发送延期风险提醒')).toBeVisible()
  await expect(page.locator('.approval-grid')).toHaveScreenshot('approval-chromium.png', {
    animations: 'disabled',
    mask: [page.locator('.deadline strong')],
  })
})

test('keeps the elicitation surface aligned with the Pencil baseline', async ({ page }) => {
  await page.route('**/api/v1/agent/tasks/TASK-1', route => ok(route, {
    taskId: 'TASK-1', status: 'waiting_input', revision: 22, persistentStatus: 'running',
    workflowProjection: { outcome: 'match', status: 'waiting_input', revision: 22 },
    pending: {
      kind: 'input', requestId: 'INPUT-1', prompt: 'Choose event settings', expiresAtUnixMs: Date.now() + 120_000,
      source: { kind: 'mcp', serverId: 'calendar.example', toolName: 'calendar.create', invocationId: 'INV-1', trust: 'untrusted' },
      form: { schemaVersion: 'dipole.agent.elicitation.v1', fields: [
        { id: 'title', label: 'Event title', type: 'text', required: true, maxLength: 120 },
        { id: 'visibility', label: 'Visibility', type: 'select', required: true, options: ['team', 'private'] },
        { id: 'notify', label: 'Notify attendees', type: 'boolean', required: false },
      ] },
    },
  }))

  await page.goto('/app/?agent=1&view=tasks&task=TASK-1&panel=input')
  await expect(page.getByText('Choose event settings')).toBeVisible()
  await expect(page.locator('.elicitation-grid')).toHaveScreenshot('elicitation-chromium.png', {
    animations: 'disabled',
    mask: [page.locator('.deadline strong')],
  })
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data }),
  })
}
