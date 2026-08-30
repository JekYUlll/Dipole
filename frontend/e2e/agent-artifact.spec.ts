import { expect, test, type Route } from '@playwright/test'

const artifactId = 'a'.repeat(64)
const metadata = { artifactId, taskId: 'TASK-ARTIFACT-1', runId: 'RUN-ARTIFACT-1', artifactType: 'analysis-report', version: 1, title: 'Project digest', mediaType: 'application/json', contentSha256: artifactId, sizeBytes: 18432, createdAtUnixMs: 1_725_000_000_000 }

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'artifact-browser-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
})

test('renders authenticated low-sensitive Artifact metadata without a download surface', async ({ page }) => {
  await page.route(`**/api/v1/agent/artifacts/${artifactId}`, route => ok(route, metadata))
  await page.goto(`/app/agent/artifacts/${artifactId}`)
  await expect(page.getByRole('heading', { name: 'Artifact metadata' })).toBeVisible()
  await expect(page.getByText('Project digest')).toBeVisible()
  await expect(page.getByText('内容与下载保持关闭')).toBeVisible()
  await expect(page.getByRole('button')).toHaveCount(0)
  await expect(page.getByText(/secret body/i)).toHaveCount(0)
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
