import { expect, test, type Route } from '@playwright/test'

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'task-create-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
    class VisualWebSocket {
      static OPEN = 1
      readyState = VisualWebSocket.OPEN
      close() { this.readyState = 3 }
      send() {}
    }
    window.WebSocket = VisualWebSocket as unknown as typeof WebSocket
  })
  await page.route('**/api/v1/users/me', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data: { uuid: 'U100', nickname: 'Owner', avatar: '', signature: '' } }),
  }))
  await page.route('**/api/v1/conversations?**', route => emptyList(route))
  await page.route('**/api/v1/contacts', route => emptyList(route))
})

test('keeps the default-off task creation surface aligned with the Pencil baseline', async ({ page }) => {
  await page.route('**/api/v1/agent/tasks**', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data: { tasks: [], nextCursor: '' } }),
  }))
  await page.goto('/app/?agent=1&view=tasks&drawer=create')

  await expect(page.getByRole('heading', { name: '创建 Agent 任务' })).toBeVisible()
  await expect(page.getByText('提交不会启用 Runtime、Tool 或外部服务。')).toBeVisible()
  await expect(page.locator('[data-agent-task-create-form]')).toHaveScreenshot('task-create-chromium.png', {
    animations: 'disabled',
  })
})

test('exposes task creation through the Agent drawer when flags are enabled', async ({ page }) => {
  await page.route('**/api/v1/agent/tasks**', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data: { tasks: [], nextCursor: '' } }),
  }))
  await page.goto('/app/?agent=1&view=tasks')
  await page.locator('[data-agent-task-create-entry]').click()
  await expect(page).toHaveURL(/view=tasks/)
  await expect(page).toHaveURL(/drawer=create/)
  await expect(page.getByRole('heading', { name: '创建 Agent 任务' })).toBeVisible()
})

async function emptyList(route: Route) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data: [] }),
  })
}
