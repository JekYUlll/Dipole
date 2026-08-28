import { IDBFactory, IDBKeyRange } from 'fake-indexeddb'
import { describe, expect, it } from 'vitest'
import type { Message } from '@/types'
import { IndexedDBSyncStore, isLocalSyncCapacityError } from './indexedDBSyncStore'
import type { SyncPage } from './syncEngine'

const syncPage = (syncSeq: number, messageID: string, conversationKey = 'direct:U1:U2'): SyncPage => {
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
      conversation_key: conversationKey,
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

  it('persists delivery replay claims across store instances', async () => {
    const factory = new IDBFactory()
    const first = new IndexedDBSyncStore(factory, IDBKeyRange, 'delivery-replay-reopen')

    await expect(first.claimDelivery('U1', 'D1')).resolves.toBe(true)
    first.close()

    const second = new IndexedDBSyncStore(factory, IDBKeyRange, 'delivery-replay-reopen')
    await expect(second.claimDelivery('U1', 'D1')).resolves.toBe(false)
    await expect(second.claimDelivery('U2', 'D1')).resolves.toBe(true)
    second.close()
  })

  it('allows only one concurrent claimant for the same delivery', async () => {
    const factory = new IDBFactory()
    const first = new IndexedDBSyncStore(factory, IDBKeyRange, 'delivery-replay-race')
    const second = new IndexedDBSyncStore(factory, IDBKeyRange, 'delivery-replay-race')

    const outcomes = await Promise.all([
      first.claimDelivery('U1', 'D1'),
      second.claimDelivery('U1', 'D1'),
    ])

    expect(outcomes.sort()).toEqual([false, true])
    first.close()
    second.close()
  })

  it('bounds delivery replay claims independently per account', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'delivery-replay-capacity', {
      deliveryReplayCapacity: 2,
    })

    await expect(store.claimDelivery('U1', 'D1')).resolves.toBe(true)
    await expect(store.claimDelivery('U1', 'D2')).resolves.toBe(true)
    await expect(store.claimDelivery('U2', 'D1')).resolves.toBe(true)
    await expect(store.claimDelivery('U1', 'D3')).resolves.toBe(true)

    await expect(store.claimDelivery('U1', 'D2')).resolves.toBe(false)
    await expect(store.claimDelivery('U1', 'D1')).resolves.toBe(true)
    await expect(store.claimDelivery('U2', 'D1')).resolves.toBe(false)
    store.close()
  })

  it('isolates accounts and clears only the requested user', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'sync-isolation')
    await store.commitPage('U1', syncPage(1, 'M1'))
    await store.commitPage('U2', syncPage(3, 'M3'))
    await store.claimDelivery('U1', 'D1')
    await store.claimDelivery('U2', 'D1')

    await store.clearUser('U1')

    await expect(store.load('U1')).resolves.toEqual({ syncSeq: 0, messages: [] })
    await expect(store.load('U2')).resolves.toEqual({
      syncSeq: 3,
      messages: [expect.objectContaining({ message_id: 'M3' })],
    })
    await expect(store.claimDelivery('U1', 'D1')).resolves.toBe(true)
    await expect(store.claimDelivery('U2', 'D1')).resolves.toBe(false)
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

  it('persists group messages and their durable message sequence across store instances', async () => {
    const factory = new IDBFactory()
    const first = new IndexedDBSyncStore(factory, IDBKeyRange, 'group-sync-reopen')
    const groupMessage = syncPage(6, 'G-M6', 'group:G1').items[0].message
    groupMessage.target_type = 1
    groupMessage.target_uuid = 'G1'
    await first.commitGroupPage('U1', 'G1', [groupMessage], 6)
    first.close()

    const second = new IndexedDBSyncStore(factory, IDBKeyRange, 'group-sync-reopen')
    await expect(second.loadGroup('U1', 'G1')).resolves.toEqual({
      messageSeq: 6,
      messages: [expect.objectContaining({ message_id: 'G-M6', message_seq: 6 })],
    })
    second.close()
  })

  it('refuses to move a durable group cursor backwards', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'group-sync-monotonic')
    await store.commitGroupPage('U1', 'G1', [syncPage(8, 'G-M8').items[0].message], 8)

    await expect(store.commitGroupPage('U1', 'G1', [syncPage(7, 'G-M7').items[0].message], 7))
      .rejects.toThrow('cannot move backwards')
    await expect(store.loadGroup('U1', 'G1')).resolves.toEqual({
      messageSeq: 8,
      messages: [expect.objectContaining({ message_id: 'G-M8' })],
    })
    store.close()
  })

  it('clears group messages and checkpoints for only the requested account', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'group-sync-clear')
    await store.commitGroupPage('U1', 'G1', [syncPage(2, 'U1-G-M2').items[0].message], 2)
    await store.commitGroupPage('U2', 'G1', [syncPage(3, 'U2-G-M3').items[0].message], 3)

    await store.clearUser('U1')

    await expect(store.loadGroup('U1', 'G1')).resolves.toEqual({ messageSeq: 0, messages: [] })
    await expect(store.loadGroup('U2', 'G1')).resolves.toEqual({
      messageSeq: 3,
      messages: [expect.objectContaining({ message_id: 'U2-G-M3' })],
    })
    store.close()
  })

  it('evicts the oldest user messages from high water to low water without moving the cursor', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'sync-capacity', {
      highWaterMessages: 3,
      lowWaterMessages: 2,
    })
    for (let syncSeq = 1; syncSeq <= 4; syncSeq += 1) {
      await store.commitPage('U1', syncPage(syncSeq, `M${syncSeq}`))
    }

    await expect(store.load('U1')).resolves.toEqual({
      syncSeq: 4,
      messages: [
        expect.objectContaining({ message_id: 'M3' }),
        expect.objectContaining({ message_id: 'M4' }),
      ],
    })
    await expect(store.getManifest('U1')).resolves.toEqual({
      syncSeq: 4,
      messageCount: 2,
      compacted: true,
    })
    store.close()
  })

  it('applies capacity independently per account', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'sync-capacity-isolation', {
      highWaterMessages: 2,
      lowWaterMessages: 1,
    })
    await store.commitPage('U2', syncPage(10, 'U2-M10'))
    for (let syncSeq = 1; syncSeq <= 3; syncSeq += 1) {
      await store.commitPage('U1', syncPage(syncSeq, `M${syncSeq}`))
    }

    await expect(store.load('U1')).resolves.toEqual({
      syncSeq: 3,
      messages: [expect.objectContaining({ message_id: 'M3' })],
    })
    await expect(store.load('U2')).resolves.toEqual({
      syncSeq: 10,
      messages: [expect.objectContaining({ message_id: 'U2-M10' })],
    })
    await expect(store.getManifest('U2')).resolves.toEqual({
      syncSeq: 10,
      messageCount: 1,
      compacted: false,
    })
    store.close()
  })

  it('preserves the newest message from each conversation when the hard limit allows it', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'sync-capacity-conversations', {
      highWaterMessages: 4,
      lowWaterMessages: 3,
      minimumMessagesPerConversation: 1,
    })
    await store.commitPage('U1', syncPage(1, 'C2-M1', 'direct:U1:U3'))
    await store.commitPage('U1', syncPage(2, 'C1-M2', 'direct:U1:U2'))
    await store.commitPage('U1', syncPage(3, 'C1-M3', 'direct:U1:U2'))
    await store.commitPage('U1', syncPage(4, 'C1-M4', 'direct:U1:U2'))
    await store.commitPage('U1', syncPage(5, 'C3-M5', 'direct:U1:U4'))

    await expect(store.load('U1')).resolves.toEqual({
      syncSeq: 5,
      messages: [
        expect.objectContaining({ message_id: 'C2-M1' }),
        expect.objectContaining({ message_id: 'C1-M4' }),
        expect.objectContaining({ message_id: 'C3-M5' }),
      ],
    })
    store.close()
  })

  it('bounds a large page at the low-water target while committing its full cursor', async () => {
    const store = new IndexedDBSyncStore(new IDBFactory(), IDBKeyRange, 'sync-capacity-large-page', {
      highWaterMessages: 100,
      lowWaterMessages: 80,
    })
    const page: SyncPage = {
      items: Array.from({ length: 250 }, (_, index) => syncPage(index + 1, `M${index + 1}`).items[0]),
      next_seq: 250,
      has_more: false,
    }

    await store.commitPage('U1', page)

    const snapshot = await store.load('U1')
    expect(snapshot.syncSeq).toBe(250)
    expect(snapshot.messages).toHaveLength(80)
    expect(snapshot.messages[0].message_id).toBe('M171')
    await expect(store.getManifest('U1')).resolves.toEqual({
      syncSeq: 250,
      messageCount: 80,
      compacted: true,
    })
    store.close()
  })

  it('upgrades a v1 database and retains its messages and cursor', async () => {
    const factory = new IDBFactory()
    await seedV1Database(factory, 'sync-upgrade')

    const store = new IndexedDBSyncStore(factory, IDBKeyRange, 'sync-upgrade')

    await expect(store.load('U1')).resolves.toEqual({
      syncSeq: 7,
      messages: [expect.objectContaining({ message_id: 'M7' })],
    })
    await expect(store.getManifest('U1')).resolves.toEqual({
      syncSeq: 7,
      messageCount: 1,
      compacted: false,
    })
    store.close()
    await Promise.resolve()
    const upgraded = await openDatabase(factory, 'sync-upgrade')
    expect(upgraded.version).toBe(4)
    expect(upgraded.transaction('messages').objectStore('messages').indexNames.contains('by_user_sync_seq')).toBe(true)
    expect(upgraded.objectStoreNames.contains('group_state')).toBe(true)
    expect(upgraded.objectStoreNames.contains('delivery_replay')).toBe(true)
    upgraded.close()
  })

  it('classifies browser quota failures without treating ordinary sync failures as capacity errors', () => {
    expect(isLocalSyncCapacityError({ name: 'QuotaExceededError' })).toBe(true)
    expect(isLocalSyncCapacityError(new Error('local quota exceeded'))).toBe(true)
    expect(isLocalSyncCapacityError(new Error('network unavailable'))).toBe(false)
  })
})

async function seedV1Database(factory: IDBFactory, databaseName: string) {
  await new Promise<void>((resolve, reject) => {
    const request = factory.open(databaseName, 1)
    request.onupgradeneeded = () => {
      const messages = request.result.createObjectStore('messages', { keyPath: 'key' })
      messages.createIndex('by_user', 'user_uuid', { unique: false })
      request.result.createObjectStore('state', { keyPath: 'user_uuid' })
    }
    request.onsuccess = () => {
      const database = request.result
      const transaction = database.transaction(['messages', 'state'], 'readwrite')
      transaction.objectStore('messages').put({
        key: 'U1\u0000M7',
        user_uuid: 'U1',
        conversation_key: 'direct:U1:U2',
        sync_seq: 7,
        message: syncPage(7, 'M7').items[0].message,
      })
      transaction.objectStore('state').put({ user_uuid: 'U1', sync_seq: 7 })
      transaction.oncomplete = () => { database.close(); resolve() }
      transaction.onerror = () => reject(transaction.error)
    }
    request.onerror = () => reject(request.error)
  })
}

function openDatabase(factory: IDBFactory, databaseName: string) {
  return new Promise<IDBDatabase>((resolve, reject) => {
    const request = factory.open(databaseName)
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}
