import { expect, test, type Route } from '@playwright/test'
import { stubChatBootstrap } from './helpers/chatBootstrap'

const timeline = {
  schemaVersion: 'dipole.agent.task_timeline.v1',
  taskId: 'TASK-VISUAL-1',
  revision: 14,
  events: [
    event('1', 'task', 'recorded', undefined, 1_700_000_000_000),
    event('2', 'context_compile', 'completed', 'conversation.read', 1_700_000_060_000),
    event('3', 'model_call', 'completed', undefined, 1_700_000_120_000),
    event('4', 'approval', 'waiting_approval', 'message.system.send', 1_700_000_180_000),
  ],
  nextCursor: '4',
}

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'timeline-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
  await stubChatBootstrap(page)
})

test('keeps the Timeline metadata surface aligned with the Pencil baseline', async ({ page }) => {
  await page.route('**/api/v1/agent/tasks/TASK-VISUAL-1/timeline**', route => ok(route, timeline))

  await page.goto('/app/?agent=1&view=tasks&task=TASK-VISUAL-1&panel=timeline')
  await expect(page.getByRole('heading', { name: '执行轨迹' })).toBeVisible()
  await expect(page.locator('[data-event-seq="4"]')).toBeVisible()
  await expect(page.getByText('model_call')).toHaveCount(0)
  await expect(page.locator('[data-agent-task-timeline]')).toHaveScreenshot('timeline-chromium.png', {
    animations: 'disabled',
  })
})

function event(
  sequence: string,
  kind: string,
  status: string,
  capabilityId: string | undefined,
  occurredAtUnixMs: number,
) {
  return {
    eventSeq: sequence,
    eventId: `EV-${sequence}`,
    taskId: 'TASK-VISUAL-1',
    runId: 'RUN-VISUAL-1',
    kind,
    status,
    occurredAtUnixMs,
    ...(capabilityId === undefined ? {} : { capabilityId }),
  }
}

async function ok(route: Route, data: unknown) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data }),
  })
}
