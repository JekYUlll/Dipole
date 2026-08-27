import { describe, expect, it, vi } from 'vitest'
import { BrowserSessionTerminator } from './sessionTermination'

function createStorage(values: Record<string, string>) {
  const entries = new Map(Object.entries(values))
  return {
    getItem: (key: string) => entries.get(key) ?? null,
    removeItem: (key: string) => { entries.delete(key) },
    entries,
  }
}

describe('BrowserSessionTerminator', () => {
  it('revokes credentials before waiting for local message cleanup', async () => {
    const storage = createStorage({
      'dipole.web.token': 'token',
      'dipole.web.user': JSON.stringify({ uuid: 'U1' }),
      'dipole.web.lastOfflineID': '42',
    })
    let releaseCleanup!: () => void
    const cleanup = new Promise<void>(resolve => { releaseCleanup = resolve })
    const clearRuntime = vi.fn()
    const redirect = vi.fn()
    const clearUserData = vi.fn(() => cleanup)
    const terminator = new BrowserSessionTerminator(storage, clearRuntime, clearUserData, redirect)

    const pending = terminator.terminate('', true)

    expect(storage.entries.size).toBe(0)
    expect(clearRuntime).toHaveBeenCalledOnce()
    expect(clearUserData).toHaveBeenCalledWith('U1')
    expect(redirect).not.toHaveBeenCalled()
    releaseCleanup()
    await pending
    expect(redirect).toHaveBeenCalledOnce()
  })

  it('coalesces cleanup while allowing a later caller to request redirect', async () => {
    const storage = createStorage({ 'dipole.web.user': JSON.stringify({ uuid: 'U1' }) })
    let releaseCleanup!: () => void
    const cleanup = new Promise<void>(resolve => { releaseCleanup = resolve })
    const redirect = vi.fn()
    const clearUserData = vi.fn(() => cleanup)
    const terminator = new BrowserSessionTerminator(storage, vi.fn(), clearUserData, redirect)

    const first = terminator.terminate('U1', false)
    const second = terminator.terminate('U1', true)

    expect(clearUserData).toHaveBeenCalledOnce()
    expect(redirect).not.toHaveBeenCalled()
    releaseCleanup()
    await Promise.all([first, second])
    expect(redirect).toHaveBeenCalledOnce()
  })

  it('contains cleanup failures and can terminate a later session', async () => {
    const storage = createStorage({ 'dipole.web.user': JSON.stringify({ uuid: 'U1' }) })
    const clearUserData = vi.fn().mockRejectedValueOnce(new Error('quota')).mockResolvedValueOnce(undefined)
    const terminator = new BrowserSessionTerminator(storage, vi.fn(), clearUserData, vi.fn())

    await expect(terminator.terminate('U1', false)).resolves.toBeUndefined()
    await expect(terminator.terminate('U2', false)).resolves.toBeUndefined()
    expect(clearUserData).toHaveBeenCalledTimes(2)
  })

  it('continues runtime and user cleanup when one storage removal fails', async () => {
    const storage = createStorage({
      'dipole.web.token': 'token',
      'dipole.web.user': JSON.stringify({ uuid: 'U1' }),
      'dipole.web.lastOfflineID': '42',
    })
    const removeItem = storage.removeItem
    storage.removeItem = key => {
      if (key === 'dipole.web.token') throw new Error('storage denied')
      removeItem(key)
    }
    const clearRuntime = vi.fn()
    const clearUserData = vi.fn().mockResolvedValue(undefined)
    const terminator = new BrowserSessionTerminator(storage, clearRuntime, clearUserData, vi.fn())

    await terminator.terminate('', false)

    expect(clearRuntime).toHaveBeenCalledOnce()
    expect(clearUserData).toHaveBeenCalledWith('U1')
    expect(storage.entries.has('dipole.web.user')).toBe(false)
    expect(storage.entries.has('dipole.web.lastOfflineID')).toBe(false)
  })
})
