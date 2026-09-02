import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import api from './index'

const originalAdapter = api.defaults.adapter

beforeAll(() => {
  const values = new Map<string, string>()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
    clear: () => values.clear(),
  })
})

afterEach(() => { api.defaults.adapter = originalAdapter })

describe('API response compatibility', () => {
  it('unwraps the existing code/data envelope', async () => {
    api.defaults.adapter = async config => ({ data: { code: 0, data: { id: 'M1' } }, status: 200, statusText: 'OK', headers: {}, config })
    await expect(api.get('/wrapped')).resolves.toEqual({ id: 'M1' })
  })

  it('preserves an authenticated raw Agent control response', async () => {
    api.defaults.adapter = async config => ({ data: { taskId: 'TASK-1', status: 'waiting_input' }, status: 200, statusText: 'OK', headers: {}, config })
    await expect(api.get('/raw')).resolves.toEqual({ taskId: 'TASK-1', status: 'waiting_input' })
  })
})
