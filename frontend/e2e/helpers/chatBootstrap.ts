import type { Page, Route } from '@playwright/test'

const me = {
  uuid: 'U100',
  nickname: 'Owner',
  avatar: '',
  signature: '',
  user_type: 0,
  status: 0,
  telephone: '',
  email: 'owner@example.test',
  is_admin: false,
  created_at: '2026-08-31T08:30:00.000Z',
}

export async function stubChatBootstrap(page: Page) {
  await page.route('**/api/v1/users/me**', route => json(route, me))
  await page.route('**/api/v1/conversations**', route => json(route, []))
  await page.route('**/api/v1/contacts**', route => json(route, []))
  await page.route('**/api/v1/devices**', route => json(route, []))
}

export async function json(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data }),
  })
}
