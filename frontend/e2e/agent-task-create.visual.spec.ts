import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'task-create-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
  await page.route('**/api/v1/users/me', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data: { uuid: 'U100', nickname: 'Owner', avatar: '', signature: '' } }),
  }))
})

test('keeps the default-off task creation surface aligned with the Pencil baseline', async ({ page }) => {
  await page.goto('/app/agent/tasks/new')

  await expect(page.getByRole('heading', { name: '创建 Agent 任务' })).toBeVisible()
  await expect(page.getByText('提交不会启用 Runtime、Tool 或外部服务。')).toBeVisible()
  await expect(page.locator('[data-agent-task-create-form]')).toHaveScreenshot('task-create-chromium.png', {
    animations: 'disabled',
  })
})

test('exposes the creation route through the authenticated IM navigation only when its flags are enabled', async ({ page }) => {
  await page.goto('/app/')

  await page.getByRole('button', { name: '创建 Agent 任务' }).click()
  await expect(page).toHaveURL(/\/app\/agent\/tasks\/new$/)
  await expect(page.getByRole('heading', { name: '创建 Agent 任务' })).toBeVisible()
})
