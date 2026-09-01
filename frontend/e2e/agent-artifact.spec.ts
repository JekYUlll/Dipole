import { expect, test, type Route } from '@playwright/test'

const artifactId = 'a'.repeat(64)
const metadata = { artifactId, taskId: 'TASK-ARTIFACT-1', runId: 'RUN-ARTIFACT-1', artifactType: 'conversation_digest', version: 1, title: 'Project digest', mediaType: 'text/markdown', contentSha256: artifactId, sizeBytes: 18432, createdAtUnixMs: 1_725_000_000_000 }
const digest = { artifactId, mediaType: 'text/markdown', content: '# Project digest\n- Ship the gateway' }

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'artifact-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
})

test('renders an authenticated digest without a download surface', async ({ page }) => {
  await page.route(`**/api/v1/agent/artifacts/${artifactId}/content`, route => ok(route, digest))
  await page.route(`**/api/v1/agent/artifacts/${artifactId}`, route => ok(route, metadata))
  await page.goto(`/app/agent/artifacts/${artifactId}`)
  await expect(page.getByRole('heading', { name: 'Artifact digest' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Project digest' })).toBeVisible()
  await expect(page.getByText('Ship the gateway')).toBeVisible()
  await expect(page.getByText('下载保持关闭')).toBeVisible()
  await expect(page.getByRole('button')).toHaveCount(0)
  await expect(page.getByText(/object key/i)).toHaveCount(0)
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
