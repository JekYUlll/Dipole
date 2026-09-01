import { expect, test, type Route } from '@playwright/test'

const user = {
  uuid: 'U100', nickname: 'Owner', avatar: '', signature: 'Build with care', user_type: 0, status: 1,
  telephone: '13800000000', email: '', is_admin: false, created_at: '2026-09-01T00:00:00Z',
}

const conversations = [
  conversation('direct:U100:U200', 'Alice', '接口契约已经更新', '09:42', 2),
  conversation('direct:U100:U300', 'Bob', '今晚同步数据库迁移', '昨天', 0),
  conversation('direct:U100:U400', 'Dipole AI', '任务摘要已准备好', '周一', 0, 2),
]

test.beforeEach(async ({ page, browserName }) => {
  test.skip(browserName !== 'chromium', 'visual baseline is canonicalized on Chromium')
  await page.addInitScript(() => {
    localStorage.setItem('dipole.web.token', 'chat-visual-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U100', nickname: 'Owner' }))
  })
  await page.route('**/api/v1/**', async route => {
    const pathname = new URL(route.request().url()).pathname
    if (pathname === '/api/v1/users/me') return ok(route, user)
    if (pathname === '/api/v1/conversations') return ok(route, conversations)
    if (pathname === '/api/v1/contacts') return ok(route, [])
    return ok(route, [])
  })
  await page.routeWebSocket('**/api/v1/ws**', ws => {
    ws.send(JSON.stringify({ type: 'connected', data: {} }))
  })
})

test('keeps the chat desktop shell aligned with the V3 surface tokens', async ({ page }) => {
  await page.goto('/app/')
  await expect(page.getByText('选择一个会话开始聊天')).toBeVisible()
  await expect(page.locator('.conv-item')).toHaveCount(3)
  await expect(page.locator('.im-container')).toHaveScreenshot('chat-desktop-chromium.png', {
    animations: 'disabled',
  })
})

function conversation(key: string, nickname: string, preview: string, time: string, unread: number, userType = 0) {
  const uuid = key.split(':').at(-1)!
  return {
    conversation_key: key,
    target_type: 0,
    target_user: {
      uuid, nickname, avatar: '', signature: '', user_type: userType, status: 1,
    },
    remark: '',
    last_message: { message_id: `M-${uuid}`, message_type: userType === 2 ? 2 : 0, preview, sent_at: `2026-09-01T${time === '昨天' ? '08:00' : '09:42'}:00Z` },
    unread_count: unread,
    last_message_seq: 100,
    read_seq: 100 - unread,
  }
}

async function ok(route: Route, data: unknown) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data }),
  })
}
