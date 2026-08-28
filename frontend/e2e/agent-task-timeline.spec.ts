import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'timeline-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
})

test('loads the owner-scoped timeline and preserves its cursor contract', async ({ page }) => {
  const requests: string[] = []
  await page.route('**/api/v1/agent/tasks/TASK-1/timeline**', async route => {
    requests.push(route.request().url())
    const url = new URL(route.request().url())
    expect(route.request().headers().authorization).toBe('Bearer timeline-browser-token')
    expect(url.searchParams.get('limit')).toBe('50')
    const pageNumber = requests.length
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ code: 0, data: {
        schemaVersion: 'dipole.agent.task_timeline.v1', taskId: 'TASK-1', revision: 4,
        events: [{ eventSeq: String(pageNumber), eventId: `EV-${pageNumber}`, taskId: 'TASK-1', runId: 'RUN-1', kind: 'model_call', status: 'completed', occurredAtUnixMs: pageNumber * 1_000 }],
        nextCursor: pageNumber === 1 ? '1' : '',
      } }),
    })
  })

  await page.goto('/app/agent/tasks/TASK-1/timeline')
  await expect(page.getByRole('heading', { name: '执行轨迹' })).toBeVisible()
  await expect(page.locator('[data-event-seq="1"]')).toBeVisible()
  await expect(page.getByText('model_call')).toHaveCount(0)
  await page.locator('[data-agent-timeline-more]').click()
  await expect(page.locator('[data-event-seq="2"]')).toBeVisible()
  expect(new URL(requests[1]).searchParams.get('after')).toBe('1')
  expect(requests).toHaveLength(2)
})
