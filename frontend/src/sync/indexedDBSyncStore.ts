import type { Message } from '@/types'
import type { LocalSyncSnapshot, LocalSyncStore, SyncPage } from './syncEngine'

const messageStoreName = 'messages'
const stateStoreName = 'state'
const userIndexName = 'by_user'

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
}

export class IndexedDBSyncStore implements LocalSyncStore {
  private database?: Promise<IDBDatabase>

  constructor(
    private readonly factory: IDBFactory = indexedDB,
    private readonly keyRange: typeof IDBKeyRange = IDBKeyRange,
    private readonly databaseName = 'dipole-web-sync-v1',
  ) {}

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
      states.put({ user_uuid: userUUID, sync_seq: page.next_seq } satisfies StoredState)
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

  close() {
    if (!this.database) return
    void this.database.then(database => database.close())
    this.database = undefined
  }

  private open(): Promise<IDBDatabase> {
    if (this.database) return this.database
    this.database = new Promise((resolve, reject) => {
      const request = this.factory.open(this.databaseName, 1)
      request.onupgradeneeded = () => {
        const database = request.result
        const messages = database.createObjectStore(messageStoreName, { keyPath: 'key' })
        messages.createIndex(userIndexName, 'user_uuid', { unique: false })
        database.createObjectStore(stateStoreName, { keyPath: 'user_uuid' })
      }
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error ?? new Error('failed to open IndexedDB'))
      request.onblocked = () => reject(new Error('IndexedDB upgrade is blocked'))
    })
    return this.database
  }
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
