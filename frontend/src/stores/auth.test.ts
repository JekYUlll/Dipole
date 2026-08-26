import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { PrivateUser } from '@/types'

const mocks = vi.hoisted(() => ({
  post: vi.fn(),
  get: vi.fn(),
  clearLocalMessages: vi.fn(),
  chat: { myUUID: '', clearLocalMessages: vi.fn(), resetRuntimeMessages: vi.fn() },
  unauthorizedHandler: undefined as undefined | (() => void | Promise<void>),
}))

vi.mock('@/api', () => ({
  default: { post: mocks.post, get: mocks.get },
  setUnauthorizedHandler: (handler: () => void | Promise<void>) => { mocks.unauthorizedHandler = handler },
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mocks.chat,
}))

import { useAuthStore } from './auth'

const user = (uuid: string): PrivateUser => ({
  uuid,
  nickname: uuid,
  avatar: '',
  signature: '',
  user_type: 0,
  status: 1,
  telephone: '',
  email: '',
  is_admin: false,
  created_at: '2026-08-27T00:00:00Z',
})

function memoryStorage(): Storage {
  const values = new Map<string, string>()
  return {
    get length() { return values.size },
    clear: () => values.clear(),
    getItem: key => values.get(key) ?? null,
    key: index => [...values.keys()][index] ?? null,
    removeItem: key => { values.delete(key) },
    setItem: (key, value) => { values.set(key, String(value)) },
  }
}

describe('auth session lifecycle', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', memoryStorage())
    localStorage.clear()
    mocks.post.mockReset()
    mocks.get.mockReset()
    mocks.chat.myUUID = ''
    mocks.chat.clearLocalMessages = vi.fn().mockResolvedValue(undefined)
    mocks.chat.resetRuntimeMessages = vi.fn()
    mocks.unauthorizedHandler = undefined
    setActivePinia(createPinia())
    window.history.replaceState({}, '', '/app/login')
  })

  it('registers 401 termination that revokes runtime state and clears the stored user', async () => {
    localStorage.setItem('dipole.web.token', 'old-token')
    localStorage.setItem('dipole.web.user', JSON.stringify(user('U1')))
    const auth = useAuthStore()

    const cleanup = mocks.unauthorizedHandler?.()

    expect(auth.token).toBe('')
    expect(auth.currentUser).toBeNull()
    expect(localStorage.getItem('dipole.web.token')).toBeNull()
    expect(mocks.chat.myUUID).toBe('')
    expect(mocks.chat.resetRuntimeMessages).toHaveBeenCalledOnce()
    await cleanup
    expect(mocks.chat.clearLocalMessages).toHaveBeenCalledWith('U1')
  })

  it('clears a different stored account before installing the new login session', async () => {
    localStorage.setItem('dipole.web.token', 'old-token')
    localStorage.setItem('dipole.web.user', JSON.stringify(user('U1')))
    mocks.post.mockResolvedValue({ token: 'new-token', user: user('U2') })
    const auth = useAuthStore()

    await auth.login('13800000000', 'password')

    expect(mocks.chat.clearLocalMessages).toHaveBeenCalledWith('U1')
    expect(auth.token).toBe('new-token')
    expect(auth.currentUser?.uuid).toBe('U2')
    expect(mocks.chat.myUUID).toBe('U2')
  })
})
