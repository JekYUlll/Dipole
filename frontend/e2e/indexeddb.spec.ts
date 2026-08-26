import { expect, test } from '@playwright/test'

const harnessURL = '/app/e2e/indexeddb.html'

test.beforeEach(async ({ page }) => {
  await page.goto(harnessURL)
  await expect(page.getByText('Dipole IndexedDB acceptance harness')).toBeVisible()
})

test('persists bounded per-user state across a real browser reopen and clears one account', async ({ page }, testInfo) => {
  const result = await page.evaluate(async databaseName => {
    return window.dipoleIndexedDBAcceptance.lifecycle(databaseName)
  }, `dipole-lifecycle-${testInfo.project.name}`)

  expect(result.compacted).toEqual({ syncSeq: 6, messageCount: 3, compacted: true })
  expect(result.beforeClearIDs).toHaveLength(3)
  expect(result.cleared).toEqual({ syncSeq: 0, messages: [] })
  expect(result.otherUserIDs).toEqual(['M-7'])
  expect(result.otherUserSyncSeq).toBe(7)
})

test('revokes credentials before delayed IndexedDB cleanup and redirects after completion', async ({ page }, testInfo) => {
  const result = await page.evaluate(async databaseName => {
    return window.dipoleIndexedDBAcceptance.sessionTermination(databaseName)
  }, `dipole-session-${testInfo.project.name}`)

  expect(result.immediate).toEqual({
    token: null,
    user: null,
    lastOfflineID: null,
    runtimeCleared: true,
    redirected: false,
    messageCount: 1,
  })
  expect(result.completed.redirected).toBe(true)
  expect(result.completed.snapshot).toEqual({ syncSeq: 0, messages: [] })
})

test('keeps an interrupted page transaction atomic across reload', async ({ page }, testInfo) => {
  const databaseName = `dipole-interrupted-${testInfo.project.name}`
  await page.evaluate(async name => {
    await window.dipoleIndexedDBAcceptance.startInterruptedWrite(name)
  }, databaseName)
  await page.reload()
  const result = await page.evaluate(async name => {
    return window.dipoleIndexedDBAcceptance.inspectInterruptedWrite(name)
  }, databaseName)

  expect([
    { syncSeq: 1, messageCount: 1 },
    { syncSeq: 2_001, messageCount: 2_001 },
  ]).toContainEqual({ syncSeq: result.syncSeq, messageCount: result.messageCount })
  expect(result.manifest.syncSeq).toBe(result.syncSeq)
  expect(result.manifest.messageCount).toBe(result.messageCount)
})

test('provides the native quota exception used by the capacity classifier', async ({ page }) => {
  const result = await page.evaluate(() => window.dipoleIndexedDBAcceptance.classifyNativeQuota())
  expect(result).toEqual({ name: 'QuotaExceededError', message: 'browser quota exceeded', classified: true })
})

test('does not advance the safe cursor when Chromium rejects a page at the real origin quota', async ({ page, context, browserName }, testInfo) => {
  test.skip(browserName !== 'chromium', 'CDP quota override is Chromium-only')
  const databaseName = `dipole-quota-${testInfo.project.name}`
  await page.evaluate(async name => {
    await window.dipoleIndexedDBAcceptance.prepareQuota(name)
  }, databaseName)
  const client = await context.newCDPSession(page)
  const origin = new URL(page.url()).origin
  const before = await client.send('Storage.getUsageAndQuota', { origin })
  await client.send('Storage.overrideQuotaForOrigin', { origin, quotaSize: before.usage + 64 * 1_024 })
  const overridden = await client.send('Storage.getUsageAndQuota', { origin })

  try {
    expect(overridden.overrideActive).toBe(true)
    const result = await page.evaluate(async name => {
      return window.dipoleIndexedDBAcceptance.exceedQuota(name)
    }, databaseName)
    if (result.errorName !== 'QuotaExceededError' && !/quota/i.test(result.errorMessage)) {
      test.skip(true, `Chromium ${browserName} reported an active CDP quota override but did not enforce it for IndexedDB`)
    }
    expect(
      result.errorName === 'QuotaExceededError' || /quota/i.test(result.errorMessage),
      JSON.stringify({ before, overridden, result }),
    ).toBe(true)
    expect(result.syncSeq).toBe(1)
    expect(result.messageIDs).toEqual(['M-1'])
  } finally {
    await client.send('Storage.overrideQuotaForOrigin', { origin })
    await page.evaluate(async name => {
      await window.dipoleIndexedDBAcceptance.cleanup(name)
    }, databaseName)
    await client.detach()
  }
})
