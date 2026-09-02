import { expect, test, type Route } from '@playwright/test'

const artifactId = 'b'.repeat(64)
const metadata = { artifactId, taskId: 'TASK-ARTIFACT-VISUAL-1', runId: 'RUN-ARTIFACT-VISUAL-1', artifactType: 'conversation_digest', version: 2, title: 'Project risk digest', mediaType: 'text/markdown', contentSha256: artifactId, sizeBytes: 18432, createdAtUnixMs: 1_725_000_000_000 }
const digest = { artifactId, mediaType: 'text/markdown', content: '# Project risk digest\n\n## Decisions\n- Ship the Gateway reader.\n\n## Follow-up\n- Verify owner access.' }

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'artifact-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
})

test('keeps the Artifact digest reader aligned with the Pencil disclosure boundary', async ({ page }) => {
  await page.route(`**/api/v1/agent/artifacts/${artifactId}/content`, route => ok(route, digest))
  await page.route(`**/api/v1/agent/artifacts/${artifactId}`, route => ok(route, metadata))
  await page.goto(`/app/agent/artifacts/${artifactId}`)
  await expect(page.locator('[data-agent-artifact-state="ready"]')).toBeVisible()
  await expect(page.locator('[data-agent-artifact-content-state="ready"]')).toBeVisible()
  await expect(page.getByText('Ship the Gateway reader.')).toBeVisible()
  await expect(page.getByText('下载保持关闭')).toBeVisible()
  await expect(page.getByRole('button')).toHaveCount(0)
  await expect(page.locator('[data-agent-artifact]')).toHaveScreenshot('agent-artifact-chromium.png', { animations: 'disabled' })
})

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
