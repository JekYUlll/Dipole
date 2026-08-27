import { IDBFactory, IDBKeyRange } from 'fake-indexeddb'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Message } from '@/types'

const api = vi.hoisted(() => ({
  get: vi.fn(),
  patch: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api', () => ({ default: api }))

import { recoverBrowserGroupMessages } from './browserSync'

const groupMessage = (seq: number): Message => ({
  id: 0,
  message_id: `G-M${seq}`,
  message_seq: seq,
  from_uuid: 'U2',
  target_uuid: 'G1',
  target_type: 1,
  message_type: 0,
  content: `G-M${seq}`,
  sent_at: '2026-08-27T12:00:00Z',
})

describe('browser group sync', () => {
  beforeEach(() => {
    vi.stubGlobal('indexedDB', new IDBFactory())
    vi.stubGlobal('IDBKeyRange', IDBKeyRange)
    api.get.mockReset()
    api.patch.mockReset().mockResolvedValue(undefined)
  })

  it('restores a committed group page without fetching it again and repairs a lost ACK', async () => {
    const userUUID = `U-${crypto.randomUUID()}`
    api.get.mockResolvedValueOnce([groupMessage(1), groupMessage(2)])
    const firstDeliver = vi.fn()

    await expect(recoverBrowserGroupMessages(
      userUUID,
      { groupUUID: 'G1', latestMessageSeq: 2 },
      firstDeliver,
    )).resolves.toEqual({ restored: 0, synchronized: 2, messageSeq: 2 })
    expect(api.get).toHaveBeenCalledWith('/api/v1/messages/group/G1?after_seq=0&limit=100')
    expect(api.patch).toHaveBeenCalledWith('/api/v1/sync/groups/G1/checkpoint', { message_seq: 2 })

    api.get.mockClear()
    api.patch.mockClear()
    const secondDeliver = vi.fn()
    await expect(recoverBrowserGroupMessages(
      userUUID,
      { groupUUID: 'G1', latestMessageSeq: 2 },
      secondDeliver,
    )).resolves.toEqual({ restored: 2, synchronized: 0, messageSeq: 2 })
    expect(secondDeliver).toHaveBeenCalledWith([
      expect.objectContaining({ message_id: 'G-M1' }),
      expect.objectContaining({ message_id: 'G-M2' }),
    ], 'local')
    expect(api.get).not.toHaveBeenCalled()
    expect(api.patch).toHaveBeenCalledWith('/api/v1/sync/groups/G1/checkpoint', { message_seq: 2 })
  })
})
