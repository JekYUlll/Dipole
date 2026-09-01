import { expect, test } from '@playwright/test'

const timeline = {
  schemaVersion: 'dipole.agent.task_timeline.v1',
  taskId: 'TASK-1',
  revision: 4,
  events: [
    { eventSeq: '1', eventId: 'EV-1', taskId: 'TASK-1', runId: 'RUN-1', kind: 'model_call', status: 'completed', occurredAtUnixMs: 1_000 },
    { eventSeq: '2', eventId: 'EV-2', taskId: 'TASK-1', runId: 'RUN-1', kind: 'tool_invocation', status: 'completed', occurredAtUnixMs: 2_000 },
    { eventSeq: '3', eventId: 'EV-3', taskId: 'TASK-1', runId: 'RUN-1', kind: 'terminal', status: 'completed', occurredAtUnixMs: 3_000 },
  ],
  nextCursor: '',
}

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'timeline-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
  await page.route('**/api/v1/agent/tasks/TASK-1/timeline**', async route => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ code: 0, data: timeline }),
    })
  })
})

test('keeps the Agent Task Timeline desktop surface aligned with the V3 baseline', async ({ page }) => {
  await page.goto('/app/agent/tasks/TASK-1/timeline')
  await expect(page.getByRole('heading', { name: '执行轨迹' })).toBeVisible()
  await expect(page.locator('.timeline-page')).toHaveScreenshot('agent-timeline-desktop-chromium.png', { animations: 'disabled' })
})

test('keeps the Agent Task Timeline mobile surface aligned with the V3 baseline', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/app/agent/tasks/TASK-1/timeline')
  await expect(page.getByRole('heading', { name: '执行轨迹' })).toBeVisible()
  await expect(page.locator('.timeline-page')).toHaveScreenshot('agent-timeline-mobile-chromium.png', { animations: 'disabled' })
})
