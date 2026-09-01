import { expect, test, type Page } from '@playwright/test'

const harnessURL = '/app/e2e/indexeddb.html'
const appURL = '/app/'

const user = {
  uuid: 'U1',
  nickname: 'User One',
  avatar: '',
  signature: '',
  user_type: 0,
  status: 1,
  telephone: '13800000000',
  email: '',
  is_admin: false,
  created_at: '2026-08-27T00:00:00Z',
}

async function prepareSharedDevice(page: Page) {
  await page.goto(harnessURL)
  await page.evaluate(async () => {
    await window.dipoleIndexedDBAcceptance.prepareSharedDevice()
    localStorage.setItem('dipole.web.token', 'shared-device-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U1', nickname: 'User One' }))
    localStorage.setItem('dipole.web.lastOfflineID', '77')
  })
}

async function inspectSharedDevice(page: Page) {
  const credentials = await page.evaluate(() => ({
    token: localStorage.getItem('dipole.web.token'),
    user: localStorage.getItem('dipole.web.user'),
    lastOfflineID: localStorage.getItem('dipole.web.lastOfflineID'),
  }))
  await page.goto(harnessURL)
  const snapshots = await page.evaluate(() => window.dipoleIndexedDBAcceptance.inspectSharedDevice())
  return { credentials, snapshots }
}

async function mockHTTP(page: Page, unauthorizedMe: boolean) {
  await page.route('**/api/v1/**', async route => {
    const pathname = new URL(route.request().url()).pathname
    if (pathname === '/api/v1/users/me' && unauthorizedMe) {
      await route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ message: 'expired' }) })
      return
    }
    const data = pathname === '/api/v1/users/me' ? user : []
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, data }) })
  })
}

function expectSharedDeviceCleanup(result: Awaited<ReturnType<typeof inspectSharedDevice>>) {
  expect(result.credentials).toEqual({ token: null, user: null, lastOfflineID: null })
  expect(result.snapshots.current).toEqual({ syncSeq: 0, messages: [] })
  expect(result.snapshots.other.syncSeq).toBe(2)
  expect(result.snapshots.other.messages.map(message => message.message_id)).toEqual(['M-2'])
}

test('HTTP 401 clears only the current account on a shared browser profile', async ({ page }) => {
  const pageErrors: string[] = []
  page.on('pageerror', error => pageErrors.push(error.message))
  await prepareSharedDevice(page)
  await mockHTTP(page, true)

  await page.goto(appURL)
  await expect(page).toHaveURL(/\/app\/login$/)

  expectSharedDeviceCleanup(await inspectSharedDevice(page))
  expect(pageErrors).toEqual([])
})

test('WS session.kicked clears only the current account on a shared browser profile', async ({ page }) => {
  await prepareSharedDevice(page)
  await mockHTTP(page, false)
  await page.routeWebSocket('**/api/v1/ws**', ws => {
    ws.send(JSON.stringify({ type: 'connected', data: {} }))
    setTimeout(() => ws.send(JSON.stringify({ type: 'session.kicked', data: { reason: 'remote login' } })), 50)
  })

  await page.goto(appURL)
  await expect(page).toHaveURL(/\/app\/login$/)

  expectSharedDeviceCleanup(await inspectSharedDevice(page))
})
