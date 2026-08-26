import type { Message } from '@/types'
import type { LocalSyncSnapshot, LocalSyncStore, SyncPage } from './syncEngine'

const messageStoreName = 'messages'
const stateStoreName = 'state'
const userIndexName = 'by_user'
const userSyncIndexName = 'by_user_sync_seq'
const databaseVersion = 2

export interface IndexedDBSyncStoreOptions {
  highWaterMessages?: number
  lowWaterMessages?: number
  minimumMessagesPerConversation?: number
}

export function isLocalSyncCapacityError(error: unknown) {
  if (!error || typeof error !== 'object') return false
  const candidate = error as { name?: unknown; message?: unknown }
  return candidate.name === 'QuotaExceededError'
    || (typeof candidate.message === 'string' && /quota/i.test(candidate.message))
}

interface StoredMessage {
  key: string
  user_uuid: string
  conversation_key: string
  sync_seq: number
  message: Message
}

interface StoredState {
  user_uuid: string
  sync_seq: number
  message_count?: number
  compacted?: boolean
}

export interface LocalSyncManifest {
  syncSeq: number
  messageCount: number
  compacted: boolean
}

export class IndexedDBSyncStore implements LocalSyncStore {
  private database?: Promise<IDBDatabase>
  private readonly highWaterMessages: number
  private readonly lowWaterMessages: number
  private readonly minimumMessagesPerConversation: number

  constructor(
    private readonly factory: IDBFactory = indexedDB,
    private readonly keyRange: typeof IDBKeyRange = IDBKeyRange,
    private readonly databaseName = 'dipole-web-sync-v1',
    options: IndexedDBSyncStoreOptions = {},
  ) {
    this.highWaterMessages = boundedInteger(options.highWaterMessages, 5_000, 1, 100_000)
    this.lowWaterMessages = boundedInteger(options.lowWaterMessages, 4_000, 0, this.highWaterMessages)
    this.minimumMessagesPerConversation = boundedInteger(
      options.minimumMessagesPerConversation,
      1,
      0,
      this.lowWaterMessages,
    )
  }

  async load(userUUID: string): Promise<LocalSyncSnapshot> {
    const database = await this.open()
    const transaction = database.transaction([messageStoreName, stateStoreName], 'readonly')
    const messagesRequest = transaction.objectStore(messageStoreName).index(userIndexName).getAll(this.keyRange.only(userUUID))
    const stateRequest = transaction.objectStore(stateStoreName).get(userUUID)
    const [records, state] = await Promise.all([
      requestResult<StoredMessage[]>(messagesRequest),
      requestResult<StoredState | undefined>(stateRequest),
      transactionResult(transaction),
    ])
    records.sort((left, right) => left.sync_seq - right.sync_seq)
    return {
      syncSeq: state?.sync_seq ?? 0,
      messages: records.map(record => record.message),
    }
  }

  async commitPage(userUUID: string, page: SyncPage): Promise<void> {
    const database = await this.open()
    const transaction = database.transaction([messageStoreName, stateStoreName], 'readwrite')
    const messages = transaction.objectStore(messageStoreName)
    const states = transaction.objectStore(stateStoreName)
    let failure: Error | undefined

    states.get(userUUID).onsuccess = event => {
      const current = (event.target as IDBRequest<StoredState | undefined>).result
      if ((current?.sync_seq ?? 0) > page.next_seq) {
        failure = new Error('local sync cursor cannot move backwards')
        transaction.abort()
        return
      }
      for (const item of page.items) {
        messages.put({
          key: `${userUUID}\u0000${item.message_uuid}`,
          user_uuid: userUUID,
          conversation_key: item.conversation_key,
          sync_seq: item.sync_seq,
          message: item.message,
        } satisfies StoredMessage)
      }
      states.put({
        user_uuid: userUUID,
        sync_seq: page.next_seq,
        message_count: current?.message_count ?? 0,
        compacted: current?.compacted ?? false,
      } satisfies StoredState)
      this.evictIfNeeded(messages, states, userUUID, page.next_seq, current?.compacted ?? false)
    }

    await transactionResult(transaction, () => failure)
  }

  async clearUser(userUUID: string): Promise<void> {
    const database = await this.open()
    const transaction = database.transaction([messageStoreName, stateStoreName], 'readwrite')
    const messages = transaction.objectStore(messageStoreName)
    const cursorRequest = messages.index(userIndexName).openKeyCursor(this.keyRange.only(userUUID))
    cursorRequest.onsuccess = () => {
      const cursor = cursorRequest.result
      if (!cursor) return
      messages.delete(cursor.primaryKey)
      cursor.continue()
    }
    transaction.objectStore(stateStoreName).delete(userUUID)
    await transactionResult(transaction)
  }

  async getManifest(userUUID: string): Promise<LocalSyncManifest> {
    const database = await this.open()
    const transaction = database.transaction([messageStoreName, stateStoreName], 'readonly')
    const stateRequest = transaction.objectStore(stateStoreName).get(userUUID)
    const countRequest = transaction.objectStore(messageStoreName).index(userIndexName).count(userUUID)
    const [state, messageCount] = await Promise.all([
      requestResult<StoredState | undefined>(stateRequest),
      requestResult(countRequest),
      transactionResult(transaction),
    ])
    return {
      syncSeq: state?.sync_seq ?? 0,
      messageCount: state?.message_count ?? messageCount,
      compacted: state?.compacted ?? false,
    }
  }

  close() {
    if (!this.database) return
    void this.database.then(database => database.close())
    this.database = undefined
  }

  private open(): Promise<IDBDatabase> {
    if (this.database) return this.database
    this.database = new Promise((resolve, reject) => {
      const request = this.factory.open(this.databaseName, databaseVersion)
      request.onupgradeneeded = event => {
        const database = request.result
        if (event.oldVersion < 1) {
          const messages = database.createObjectStore(messageStoreName, { keyPath: 'key' })
          messages.createIndex(userIndexName, 'user_uuid', { unique: false })
          database.createObjectStore(stateStoreName, { keyPath: 'user_uuid' })
        }
        if (event.oldVersion < 2) {
          request.transaction!.objectStore(messageStoreName)
            .createIndex(userSyncIndexName, ['user_uuid', 'sync_seq'], { unique: false })
        }
      }
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error ?? new Error('failed to open IndexedDB'))
      request.onblocked = () => reject(new Error('IndexedDB upgrade is blocked'))
    })
    return this.database
  }

  private evictIfNeeded(
    messages: IDBObjectStore,
    states: IDBObjectStore,
    userUUID: string,
    syncSeq: number,
    previouslyCompacted: boolean,
  ) {
    const range = this.keyRange.bound(
      [userUUID, 0],
      [userUUID, Number.MAX_SAFE_INTEGER],
    )
    const index = messages.index(userSyncIndexName)
    const countRequest = index.count(range)
    countRequest.onsuccess = () => {
      let remaining = countRequest.result > this.highWaterMessages
        ? countRequest.result - this.lowWaterMessages
        : 0
      if (remaining === 0) {
        states.put({
          user_uuid: userUUID,
          sync_seq: syncSeq,
          message_count: countRequest.result,
          compacted: previouslyCompacted,
        } satisfies StoredState)
        return
      }
      const deleteCount = remaining
      const recordsRequest = index.getAll(range)
      recordsRequest.onsuccess = () => {
        const records = recordsRequest.result as StoredMessage[]
        const conversationCounts = new Map<string, number>()
        for (const record of records) {
          conversationCounts.set(record.conversation_key, (conversationCounts.get(record.conversation_key) ?? 0) + 1)
        }

        const deletedKeys = new Set<string>()
        for (const record of records) {
          if (remaining === 0) break
          const conversationCount = conversationCounts.get(record.conversation_key) ?? 0
          if (conversationCount <= this.minimumMessagesPerConversation) continue
          messages.delete(record.key)
          deletedKeys.add(record.key)
          conversationCounts.set(record.conversation_key, conversationCount - 1)
          remaining -= 1
        }

        // Extremely many conversations may exceed the hard cap even after preserving one each.
        for (const record of records) {
          if (remaining === 0) break
          if (deletedKeys.has(record.key)) continue
          messages.delete(record.key)
          remaining -= 1
        }
        states.put({
          user_uuid: userUUID,
          sync_seq: syncSeq,
          message_count: countRequest.result - deleteCount,
          compacted: true,
        } satisfies StoredState)
      }
    }
  }
}

function boundedInteger(value: number | undefined, fallback: number, minimum: number, maximum: number) {
  const candidate = Number.isSafeInteger(value) ? value as number : fallback
  return Math.min(maximum, Math.max(minimum, candidate))
}

function requestResult<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error ?? new Error('IndexedDB request failed'))
  })
}

function transactionResult(transaction: IDBTransaction, failure?: () => Error | undefined): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.oncomplete = () => resolve()
    transaction.onerror = () => reject(failure?.() ?? transaction.error ?? new Error('IndexedDB transaction failed'))
    transaction.onabort = () => reject(failure?.() ?? transaction.error ?? new Error('IndexedDB transaction aborted'))
  })
}
