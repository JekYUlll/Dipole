import { expect, test, type Route } from '@playwright/test'
import { stubChatBootstrap } from './helpers/chatBootstrap'

const artifactId = 'a'.repeat(64)
const metadata = {
  artifactId, taskId: 'TASK-ARTIFACT-1', runId: 'RUN-ARTIFACT-1', artifactType: 'conversation_digest',
  version: 1, title: 'Project digest', mediaType: 'text/markdown', contentSha256: artifactId,
  sizeBytes: 18432, createdAtUnixMs: 1_725_000_000_000,
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'artifact-inbox-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
  await stubChatBootstrap(page)
})

test('lists owner artifact metadata without fetching digest content', async ({ page }) => {
  const requested: string[] = []
  await page.route('**/api/v1/agent/artifacts**', async route => {
    const path = new URL(route.request().url()).pathname
    requested.push(path)
    if (path === '/api/v1/agent/artifacts') {
      await ok(route, { artifacts: [metadata], nextCursor: '' })
      return
    }
    await route.fulfill({ status: 404 })
  })

  await page.goto('/app/?agent=1&view=artifacts')
  await expect(page.locator('[data-agent-artifacts-view]')).toBeVisible()
  await expect(page.getByText('Project digest')).toBeVisible()
  await expect(page.getByText('Ship the gateway')).toHaveCount(0)
  await expect(page.getByText(/object key/i)).toHaveCount(0)
  expect(requested.every(path => path === '/api/v1/agent/artifacts')).toBe(true)
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
