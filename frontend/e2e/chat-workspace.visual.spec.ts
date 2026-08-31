import { expect, test, type Route } from '@playwright/test'

const owner = {
  uuid: 'U100', nickname: 'Owner', avatar: '', signature: '', user_type: 0, status: 0,
  telephone: '', email: 'owner@example.test', is_admin: false, created_at: '2026-09-01T00:00:00.000Z',
}

const collaborator = {
  uuid: 'U200', nickname: 'Kai', avatar: '', signature: 'Design systems', user_type: 0, status: 0,
}

const conversation = {
  conversation_key: 'direct:U100:U200', target_type: 0, target_user: collaborator, remark: '',
  last_message: { message_id: 'M2', message_type: 0, preview: 'I have added the review notes.', sent_at: '2026-09-01T09:30:00.000Z', sender_uuid: 'U200' },
  unread_count: 2, last_message_seq: 12, read_seq: 10,
}

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'chat-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
    class VisualWebSocket {
      static OPEN = 1
      readyState = VisualWebSocket.OPEN
      close() { this.readyState = 3 }
      send() {}
    }
    window.WebSocket = VisualWebSocket as unknown as typeof WebSocket
  })
  await page.route('**/api/v1/users/me', route => ok(route, owner))
  await page.route('**/api/v1/conversations?**', route => ok(route, [conversation]))
  await page.route('**/api/v1/contacts', route => ok(route, [{ user: collaborator, remark: '', status: 0 }]))
  await page.route('**/api/v1/contacts/applications?**', route => ok(route, []))
  await page.route('**/api/v1/users/me/devices', route => ok(route, []))
  await page.route('**/api/v1/messages/offline?**', route => ok(route, []))
  await page.route('**/api/v1/sync/groups/checkpoints?**', route => ok(route, []))
  await page.route('**/api/v1/messages/direct/U200?**', route => ok(route, [
    message('M1', 'U100', 'Draft is ready for a quick review.', '2026-09-01T09:20:00.000Z', 11),
    message('M2', 'U200', 'I have added the review notes.', '2026-09-01T09:30:00.000Z', 12),
  ]))
  await page.route('**/api/v1/conversations/direct/U200/read', route => ok(route, {}))
})

test('keeps the V3 Chat workspace aligned with the primary collaboration canvas', async ({ page }) => {
  await page.goto('/app/')
  await expect(page.getByText('Kai', { exact: true })).toBeVisible()
  await page.getByText('Kai', { exact: true }).click()
  await expect(page.getByRole('button', { name: '发送' })).toBeVisible()
  await expect(page.locator('.im-container')).toHaveScreenshot('chat-workspace-chromium.png', {
    animations: 'disabled',
  })
})

function message(messageId: string, fromUUID: string, content: string, sentAt: string, sequence: number) {
  return {
    id: sequence, message_id: messageId, message_seq: sequence, from_uuid: fromUUID,
    target_uuid: fromUUID === 'U100' ? 'U200' : 'U100', target_type: 0, message_type: 0,
    content, sent_at: sentAt,
  }
}

async function ok(route: Route, data: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
}
