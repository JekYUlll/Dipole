import { describe, expect, it, vi } from 'vitest'
import type { Message } from '@/types'
import {
  MessageSyncEngine,
  type LocalSyncSnapshot,
  type LocalSyncStore,
  type SyncPage,
  type SyncTransport,
} from './syncEngine'

const message = (id: string, seq: number): Message => ({
  id: 0,
  message_id: id,
  message_seq: seq,
  from_uuid: 'U2',
  target_uuid: 'U1',
  target_type: 0,
  message_type: 0,
  content: id,
  sent_at: '2026-08-27T12:00:00Z',
})

const page = (from: number, to: number, hasMore: boolean): SyncPage => ({
  items: Array.from({ length: to - from }, (_, offset) => {
    const syncSeq = from + offset + 1
    return {
      sync_seq: syncSeq,
      conversation_key: 'direct:U1:U2',
      message_uuid: `M${syncSeq}`,
      message_seq: syncSeq,
      message: message(`M${syncSeq}`, syncSeq),
    }
  }),
  next_seq: to,
  has_more: hasMore,
})

class MemoryStore implements LocalSyncStore {
  snapshot: LocalSyncSnapshot
  events: string[]

  constructor(snapshot: LocalSyncSnapshot, events: string[]) {
    this.snapshot = snapshot
    this.events = events
  }

  async load() {
    this.events.push('load')
    return this.snapshot
  }

  async commitPage(_userUUID: string, value: SyncPage) {
    this.events.push(`commit:${value.next_seq}`)
    this.snapshot = {
      syncSeq: value.next_seq,
      messages: [...this.snapshot.messages, ...value.items.map(item => item.message)],
    }
  }

  async clearUser() {}
}

describe('MessageSyncEngine', () => {
  it('restores local messages before fetching and acknowledges only committed pages', async () => {
    const events: string[] = []
    const store = new MemoryStore({ syncSeq: 2, messages: [message('M2', 2)] }, events)
    const transport: SyncTransport = {
      list: vi.fn(async (afterSeq) => {
        events.push(`list:${afterSeq}`)
        return page(afterSeq, 4, false)
      }),
      acknowledge: vi.fn(async (syncSeq) => { events.push(`ack:${syncSeq}`) }),
    }
    const delivered: string[][] = []

    const result = await new MessageSyncEngine(store, transport).recover('U1', messages => {
      events.push(`deliver:${messages.map(item => item.message_id).join(',')}`)
      delivered.push(messages.map(item => item.message_id))
    })

    expect(result).toEqual({ restored: 1, synchronized: 2, syncSeq: 4 })
    expect(delivered).toEqual([['M2'], ['M3', 'M4']])
    expect(events).toEqual([
      'load',
      'deliver:M2',
      'ack:2',
      'list:2',
      'commit:4',
      'deliver:M3,M4',
      'ack:4',
    ])
  })

  it('continues through bounded pages using the locally committed cursor', async () => {
    const events: string[] = []
    const store = new MemoryStore({ syncSeq: 0, messages: [] }, events)
    const pages = [page(0, 2, true), page(2, 3, false)]
    const transport: SyncTransport = {
      list: vi.fn(async () => pages.shift()!),
      acknowledge: vi.fn(async () => {}),
    }

    const result = await new MessageSyncEngine(store, transport, { pageSize: 2 }).recover('U1', () => {})

    expect(result).toEqual({ restored: 0, synchronized: 3, syncSeq: 3 })
    expect(transport.list).toHaveBeenNthCalledWith(1, 0, 2)
    expect(transport.list).toHaveBeenNthCalledWith(2, 2, 2)
  })

  it('does not expose or acknowledge a page when local persistence fails', async () => {
    const store = new MemoryStore({ syncSeq: 0, messages: [] }, [])
    store.commitPage = vi.fn(async () => { throw new Error('quota exceeded') })
    const transport: SyncTransport = {
      list: vi.fn(async () => page(0, 1, false)),
      acknowledge: vi.fn(async () => {}),
    }
    const deliver = vi.fn()

    await expect(new MessageSyncEngine(store, transport).recover('U1', deliver)).rejects.toThrow('quota exceeded')
    expect(deliver).not.toHaveBeenCalled()
    expect(transport.acknowledge).not.toHaveBeenCalled()
  })

  it('rejects a non-advancing page before persistence', async () => {
    const store = new MemoryStore({ syncSeq: 7, messages: [] }, [])
    const transport: SyncTransport = {
      list: vi.fn(async () => ({ items: [], next_seq: 7, has_more: true })),
      acknowledge: vi.fn(async () => {}),
    }

    await expect(new MessageSyncEngine(store, transport).recover('U1', () => {}))
      .rejects.toThrow('did not advance')
    expect(store.snapshot.syncSeq).toBe(7)
  })
})
