import { IDBFactory, IDBKeyRange } from 'fake-indexeddb'
import { describe, expect, it } from 'vitest'
import type { Message } from '@/types'
import { IndexedDBSyncStore } from './indexedDBSyncStore'
import type { SyncPage } from './syncEngine'

const syncPage = (syncSeq: number, messageID: string): SyncPage => {
  const message: Message = {
    id: 0,
    message_id: messageID,
    message_seq: syncSeq,
    from_uuid: 'U2',
    target_uuid: 'U1',
    target_type: 0,
    message_type: 0,
    content: messageID,
    sent_at: '2026-08-27T12:00:00Z',
  }
  return {
    items: [{
      sync_seq: syncSeq,
      conversation_key: 'direct:U1:U2',
      message_uuid: messageID,
      message_seq: syncSeq,
      message,
    }],
    next_seq: syncSeq,
    has_more: false,
  }
}

describe('IndexedDBSyncStore', () => {
  it('persists messages and the safe cursor across store instances', async () => {
    const factory = new IDBFactory()
    const first = new IndexedDBSyncStore(factory, IDBKeyRange, 'sync-reopen')
    await first.commitPage('U1', syncPage(8, 'M8'))
    first.close()

    const second = new IndexedDBSyncStore(factory, IDBKeyRange, 'sync-reopen')
    await expect(second.load('U1')).resolves.toEqual({
      syncSeq: 8,
      messages: [expect.objectContaining({ message_id: 'M8', message_seq: 8 })],
    })
    second.close()
  })

  it('isolates accounts and clears only the requested user', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'sync-isolation')
    await store.commitPage('U1', syncPage(1, 'M1'))
    await store.commitPage('U2', syncPage(3, 'M3'))

    await store.clearUser('U1')

    await expect(store.load('U1')).resolves.toEqual({ syncSeq: 0, messages: [] })
    await expect(store.load('U2')).resolves.toEqual({
      syncSeq: 3,
      messages: [expect.objectContaining({ message_id: 'M3' })],
    })
    store.close()
  })

  it('refuses to move the locally durable cursor backwards', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'sync-monotonic')
    await store.commitPage('U1', syncPage(9, 'M9'))

    await expect(store.commitPage('U1', syncPage(8, 'M8'))).rejects.toThrow('cannot move backwards')
    await expect(store.load('U1')).resolves.toEqual({
      syncSeq: 9,
      messages: [expect.objectContaining({ message_id: 'M9' })],
    })
    store.close()
  })
})
