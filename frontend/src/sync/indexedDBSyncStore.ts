import type { Message } from '@/types'
import type { LocalGroupSyncSnapshot, LocalGroupSyncStore } from './groupSyncEngine'
import type { LocalSyncSnapshot, LocalSyncStore, SyncPage } from './syncEngine'

const messageStoreName = 'messages'
const stateStoreName = 'state'
const groupStateStoreName = 'group_state'
const deliveryReplayStoreName = 'delivery_replay'
const userIndexName = 'by_user'
const userSyncIndexName = 'by_user_sync_seq'
const userConversationSyncIndexName = 'by_user_conversation_sync_seq'
const userDeliveryIndexName = 'by_user_delivery'
const userDeliverySequenceIndexName = 'by_user_sequence'
const databaseVersion = 4

export interface IndexedDBSyncStoreOptions {
  highWaterMessages?: number
  lowWaterMessages?: number
  minimumMessagesPerConversation?: number
  deliveryReplayCapacity?: number
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

interface StoredGroupState {
  key: string
  user_uuid: string
  group_uuid: string
  message_seq: number
}

interface StoredDeliveryReplay {
  sequence?: number
  user_uuid: string
  delivery_id: string
}

export interface LocalSyncManifest {
  syncSeq: number
  messageCount: number
  compacted: boolean
}

export class IndexedDBSyncStore implements LocalSyncStore, LocalGroupSyncStore {
  private database?: Promise<IDBDatabase>
  private readonly highWaterMessages: number
  private readonly lowWaterMessages: number
  private readonly minimumMessagesPerConversation: number
  private readonly deliveryReplayCapacity: number

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
    this.deliveryReplayCapacity = boundedInteger(options.deliveryReplayCapacity, 4_096, 1, 100_000)
  }

  async claimDelivery(userUUID: string, deliveryID: string): Promise<boolean> {
    const normalizedUserUUID = userUUID.trim()
    const normalizedDeliveryID = deliveryID.trim()
    if (!normalizedUserUUID || !normalizedDeliveryID) throw new Error('delivery replay identity is required')

    const database = await this.open()
    const transaction = database.transaction(deliveryReplayStoreName, 'readwrite')
    const replays = transaction.objectStore(deliveryReplayStoreName)
    const deliveries = replays.index(userDeliveryIndexName)
    let accepted = false

    deliveries.get([normalizedUserUUID, normalizedDeliveryID]).onsuccess = event => {
      const existing = (event.target as IDBRequest<StoredDeliveryReplay | undefined>).result
      if (existing) return

      accepted = true
      replays.add({
        user_uuid: normalizedUserUUID,
        delivery_id: normalizedDeliveryID,
      } satisfies StoredDeliveryReplay)
      const countRequest = replays.index(userIndexName).count(normalizedUserUUID)
      countRequest.onsuccess = () => {
        let excess = countRequest.result - this.deliveryReplayCapacity
        if (excess <= 0) return
        const range = this.keyRange.bound(
          [normalizedUserUUID, 0],
          [normalizedUserUUID, Number.MAX_SAFE_INTEGER],
        )
        const cursorRequest = replays.index(userDeliverySequenceIndexName).openKeyCursor(range)
        cursorRequest.onsuccess = () => {
          const cursor = cursorRequest.result
          if (!cursor || excess <= 0) return
          replays.delete(cursor.primaryKey)
          excess -= 1
          cursor.continue()
        }
      }
    }

    await transactionResult(transaction)
    return accepted
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

  async loadGroup(userUUID: string, groupUUID: string): Promise<LocalGroupSyncSnapshot> {
    const database = await this.open()
    const transaction = database.transaction([messageStoreName, groupStateStoreName], 'readonly')
    const conversationKey = `group:${groupUUID}`
    const range = this.keyRange.bound(
      [userUUID, conversationKey, 0],
      [userUUID, conversationKey, Number.MAX_SAFE_INTEGER],
    )
    const messagesRequest = transaction.objectStore(messageStoreName)
      .index(userConversationSyncIndexName).getAll(range)
    const stateRequest = transaction.objectStore(groupStateStoreName).get(groupStateKey(userUUID, groupUUID))
    const [records, state] = await Promise.all([
      requestResult<StoredMessage[]>(messagesRequest),
      requestResult<StoredGroupState | undefined>(stateRequest),
      transactionResult(transaction),
    ])
    return {
      messageSeq: state?.message_seq ?? 0,
      messages: records.map(record => record.message),
    }
  }

  async commitGroupPage(userUUID: string, groupUUID: string, page: Message[], messageSeq: number): Promise<void> {
    const database = await this.open()
    const transaction = database.transaction(
      [messageStoreName, stateStoreName, groupStateStoreName],
      'readwrite',
    )
    const messages = transaction.objectStore(messageStoreName)
    const states = transaction.objectStore(stateStoreName)
    const groupStates = transaction.objectStore(groupStateStoreName)
    const stateRequest = states.get(userUUID)
    const groupStateRequest = groupStates.get(groupStateKey(userUUID, groupUUID))
    let currentState: StoredState | undefined
    let currentGroupState: StoredGroupState | undefined
    let loaded = 0
    let failure: Error | undefined

    const commit = () => {
      loaded += 1
      if (loaded < 2) return
      if ((currentGroupState?.message_seq ?? 0) > messageSeq) {
        failure = new Error('local group sync cursor cannot move backwards')
        transaction.abort()
        return
      }
      const conversationKey = `group:${groupUUID}`
      for (const message of page) {
        messages.put({
          key: `${userUUID}\u0000${message.message_id}`,
          user_uuid: userUUID,
          conversation_key: conversationKey,
          sync_seq: message.message_seq!,
          message,
        } satisfies StoredMessage)
      }
      groupStates.put({
        key: groupStateKey(userUUID, groupUUID),
        user_uuid: userUUID,
        group_uuid: groupUUID,
        message_seq: messageSeq,
      } satisfies StoredGroupState)
      const syncSeq = currentState?.sync_seq ?? 0
      states.put({
        user_uuid: userUUID,
        sync_seq: syncSeq,
        message_count: currentState?.message_count ?? 0,
        compacted: currentState?.compacted ?? false,
      } satisfies StoredState)
      this.evictIfNeeded(messages, states, userUUID, syncSeq, currentState?.compacted ?? false)
    }
    stateRequest.onsuccess = () => { currentState = stateRequest.result; commit() }
    groupStateRequest.onsuccess = () => { currentGroupState = groupStateRequest.result; commit() }

    await transactionResult(transaction, () => failure)
  }

  async clearUser(userUUID: string): Promise<void> {
    const database = await this.open()
    const transaction = database.transaction(
      [messageStoreName, stateStoreName, groupStateStoreName, deliveryReplayStoreName],
      'readwrite',
    )
    const messages = transaction.objectStore(messageStoreName)
    const cursorRequest = messages.index(userIndexName).openKeyCursor(this.keyRange.only(userUUID))
    cursorRequest.onsuccess = () => {
      const cursor = cursorRequest.result
      if (!cursor) return
      messages.delete(cursor.primaryKey)
      cursor.continue()
    }
    transaction.objectStore(stateStoreName).delete(userUUID)
    const groupStates = transaction.objectStore(groupStateStoreName)
    const groupCursorRequest = groupStates.index(userIndexName).openKeyCursor(this.keyRange.only(userUUID))
    groupCursorRequest.onsuccess = () => {
      const cursor = groupCursorRequest.result
      if (!cursor) return
      groupStates.delete(cursor.primaryKey)
      cursor.continue()
    }
    const deliveryReplays = transaction.objectStore(deliveryReplayStoreName)
    const deliveryCursorRequest = deliveryReplays.index(userIndexName).openKeyCursor(userUUID)
    deliveryCursorRequest.onsuccess = () => {
      const cursor = deliveryCursorRequest.result
      if (!cursor) return
      deliveryReplays.delete(cursor.primaryKey)
      cursor.continue()
    }
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
        if (event.oldVersion < 3) {
          request.transaction!.objectStore(messageStoreName)
            .createIndex(userConversationSyncIndexName, ['user_uuid', 'conversation_key', 'sync_seq'], { unique: false })
          const groupStates = database.createObjectStore(groupStateStoreName, { keyPath: 'key' })
          groupStates.createIndex(userIndexName, 'user_uuid', { unique: false })
        }
        if (event.oldVersion < 4) {
          const deliveryReplays = database.createObjectStore(deliveryReplayStoreName, {
            keyPath: 'sequence',
            autoIncrement: true,
          })
          deliveryReplays.createIndex(userIndexName, 'user_uuid', { unique: false })
          deliveryReplays.createIndex(userDeliveryIndexName, ['user_uuid', 'delivery_id'], { unique: true })
          deliveryReplays.createIndex(userDeliverySequenceIndexName, ['user_uuid', 'sequence'], { unique: true })
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

function groupStateKey(userUUID: string, groupUUID: string) {
  return `${userUUID}\u0000${groupUUID}`
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
