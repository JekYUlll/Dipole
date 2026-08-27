import { describe, expect, it, vi } from 'vitest'
import type { Message } from '@/types'
import { drainLegacyOffline } from './legacyOffline'

const message = (id: number): Message => ({
  id,
  message_id: `M${id}`,
  message_seq: id,
  from_uuid: 'U2',
  target_uuid: 'U1',
  target_type: 0,
  message_type: 0,
  content: `M${id}`,
  sent_at: '2026-08-27T12:00:00Z',
})

describe('drainLegacyOffline', () => {
  it('commits each advancing page and continues from its safe ID', async () => {
    const list = vi.fn()
      .mockResolvedValueOnce([message(6), message(7)])
      .mockResolvedValueOnce([message(9)])
    const committed: number[] = []

    const result = await drainLegacyOffline(5, list, (_items, nextID) => { committed.push(nextID) }, { pageSize: 2 })

    expect(result.map(item => item.id)).toEqual([6, 7, 9])
    expect(list).toHaveBeenNthCalledWith(1, 5, 2)
    expect(list).toHaveBeenNthCalledWith(2, 7, 2)
    expect(committed).toEqual([7, 9])
  })

  it('rejects a full page that cannot advance the legacy cursor', async () => {
    await expect(drainLegacyOffline(7, async () => [message(7)], () => {}, { pageSize: 1 }))
      .rejects.toThrow('did not advance')
  })

  it('bounds the number of pages drained in one synchronization run', async () => {
    let nextID = 0
    await expect(drainLegacyOffline(0, async () => [message(++nextID)], () => {}, { pageSize: 1, maxPages: 2 }))
      .rejects.toThrow('page limit exceeded')
  })
})
