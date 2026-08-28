import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
})

for (const state of ['idle', 'loading', 'results', 'empty', 'error']) {
  test(`keeps the ${state} Search Workspace state aligned with the Pencil baseline`, async ({ page }) => {
    await page.goto(`/app/e2e/search.html?state=${state}`)
    if (state === 'idle') await expect(page.getByRole('heading', { name: '开始搜索' })).toBeVisible()
    if (state === 'loading') await expect(page.locator('[aria-label="正在搜索"]')).toBeVisible()
    if (state === 'results') await expect(page.locator('[data-search-result="M9"]')).toBeVisible()
    if (state === 'empty') await expect(page.getByRole('heading', { name: '没有找到相关消息' })).toBeVisible()
    if (state === 'error') await expect(page.getByRole('heading', { name: '搜索服务未响应' })).toBeVisible()
    await expect(page.locator('.search-workspace')).toHaveScreenshot(`search-${state}-chromium.png`, {
      animations: 'disabled',
    })
  })
}
