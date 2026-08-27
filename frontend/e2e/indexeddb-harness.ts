import { BrowserSessionTerminator } from '../src/session/sessionTermination'
import { IndexedDBSyncStore, isLocalSyncCapacityError } from '../src/sync/indexedDBSyncStore'
import type { SyncPage } from '../src/sync/syncEngine'
import type { Message } from '../src/types'

function message(id: number, conversation: string, content = `message-${id}`): Message {
  return {
    id,
    message_id: `M-${id}`,
    message_seq: id,
    from_uuid: 'U-sender',
    target_uuid: conversation,
    target_type: conversation.startsWith('G') ? 1 : 0,
    message_type: 0,
    content,
    sent_at: new Date(1_700_000_000_000 + id).toISOString(),
  }
}

function page(items: Array<{ id: number; conversation: string; content?: string }>, nextSeq: number): SyncPage {
  return {
    items: items.map(({ id, conversation, content }) => ({
      sync_seq: id,
      conversation_key: conversation,
      message_seq: id,
      message_uuid: `M-${id}`,
      message: message(id, conversation, content),
    })),
    next_seq: nextSeq,
    has_more: false,
  }
}

async function deleteDatabase(name: string) {
  await new Promise<void>((resolve, reject) => {
    const request = indexedDB.deleteDatabase(name)
    request.onsuccess = () => resolve()
    request.onerror = () => reject(request.error)
    request.onblocked = () => reject(new Error(`delete blocked for ${name}`))
  })
}

let interruptedWritePending = false

const acceptance = {
  async lifecycle(databaseName: string) {
    const options = { highWaterMessages: 5, lowWaterMessages: 3, minimumMessagesPerConversation: 1 }
    const first = new IndexedDBSyncStore(indexedDB, IDBKeyRange, databaseName, options)
    await first.commitPage('U1', page([
      { id: 1, conversation: 'direct:U1:U2' },
      { id: 2, conversation: 'direct:U1:U3' },
      { id: 3, conversation: 'direct:U1:U2' },
      { id: 4, conversation: 'direct:U1:U3' },
      { id: 5, conversation: 'direct:U1:U2' },
      { id: 6, conversation: 'direct:U1:U3' },
    ], 6))
    await first.commitPage('U2', page([{ id: 7, conversation: 'direct:U2:U4' }], 7))
    const compacted = await first.getManifest('U1')
    first.close()

    const reopened = new IndexedDBSyncStore(indexedDB, IDBKeyRange, databaseName, options)
    const beforeClear = await reopened.load('U1')
    await reopened.clearUser('U1')
    const cleared = await reopened.load('U1')
    const otherUser = await reopened.load('U2')
    reopened.close()
    await deleteDatabase(databaseName)
    return {
      compacted,
      beforeClearIDs: beforeClear.messages.map(item => item.message_id),
      cleared,
      otherUserIDs: otherUser.messages.map(item => item.message_id),
      otherUserSyncSeq: otherUser.syncSeq,
    }
  },

  async sessionTermination(databaseName: string) {
    const store = new IndexedDBSyncStore(indexedDB, IDBKeyRange, databaseName)
    await store.commitPage('U1', page([{ id: 1, conversation: 'direct:U1:U2' }], 1))
    localStorage.setItem('dipole.web.token', 'test-token')
    localStorage.setItem('dipole.web.user', JSON.stringify({ uuid: 'U1' }))
    localStorage.setItem('dipole.web.lastOfflineID', '123')
    let runtimeCleared = false
    let redirected = false
    const terminator = new BrowserSessionTerminator(
      localStorage,
      () => { runtimeCleared = true },
      async userUUID => {
        await new Promise(resolve => setTimeout(resolve, 50))
        await store.clearUser(userUUID)
      },
      () => { redirected = true },
    )
    const termination = terminator.terminate('U1', true)
    const immediate = {
      token: localStorage.getItem('dipole.web.token'),
      user: localStorage.getItem('dipole.web.user'),
      lastOfflineID: localStorage.getItem('dipole.web.lastOfflineID'),
      runtimeCleared,
      redirected,
      messageCount: (await store.load('U1')).messages.length,
    }
    await termination
    const completed = {
      redirected,
      snapshot: await store.load('U1'),
    }
    store.close()
    await deleteDatabase(databaseName)
    return { immediate, completed }
  },

  async startInterruptedWrite(databaseName: string) {
    const store = new IndexedDBSyncStore(indexedDB, IDBKeyRange, databaseName, {
      highWaterMessages: 5_000,
      lowWaterMessages: 4_000,
    })
    await store.commitPage('U1', page([{ id: 1, conversation: 'direct:U1:U2' }], 1))
    const items = Array.from({ length: 2_000 }, (_, index) => ({
      id: index + 2,
      conversation: `direct:U1:U${index % 4 + 2}`,
      content: 'x'.repeat(1_024),
    }))
    interruptedWritePending = true
    void store.commitPage('U1', page(items, 2_001))
      .catch(() => undefined)
      .finally(() => { interruptedWritePending = false })
  },

  interruptedWritePending() {
    return interruptedWritePending
  },

  async inspectInterruptedWrite(databaseName: string) {
    const store = new IndexedDBSyncStore(indexedDB, IDBKeyRange, databaseName, {
      highWaterMessages: 5_000,
      lowWaterMessages: 4_000,
    })
    const snapshot = await store.load('U1')
    const manifest = await store.getManifest('U1')
    store.close()
    await deleteDatabase(databaseName)
    return { syncSeq: snapshot.syncSeq, messageCount: snapshot.messages.length, manifest }
  },

  async prepareQuota(databaseName: string) {
    const store = new IndexedDBSyncStore(indexedDB, IDBKeyRange, databaseName)
    await store.commitPage('U1', page([{ id: 1, conversation: 'direct:U1:U2' }], 1))
    store.close()
  },

  async exceedQuota(databaseName: string) {
    const store = new IndexedDBSyncStore(indexedDB, IDBKeyRange, databaseName)
    let errorName = ''
    let errorMessage = ''
    const chunks: string[] = []
    for (let index = 0; index < 8; index += 1) {
      const bytes = crypto.getRandomValues(new Uint8Array(64 * 1_024))
      chunks.push(btoa(String.fromCharCode(...bytes)))
    }
    try {
      await store.commitPage('U1', page([{
        id: 2,
        conversation: 'direct:U1:U2',
        content: chunks.join(''),
      }], 2))
    } catch (error) {
      const candidate = error as Error
      errorName = candidate.name
      errorMessage = candidate.message
    }
    const snapshot = await store.load('U1')
    store.close()
    return {
      errorName,
      errorMessage,
      syncSeq: snapshot.syncSeq,
      messageIDs: snapshot.messages.map(item => item.message_id),
    }
  },

  async fillUntilStorageRejected(databaseName: string) {
    const store = new IndexedDBSyncStore(indexedDB, IDBKeyRange, databaseName, {
      highWaterMessages: 100_000,
      lowWaterMessages: 90_000,
    })
    await store.commitPage('U1', page([{ id: 1, conversation: 'direct:U1:U2' }], 1))
    let lastCommittedSeq = 1
    for (let id = 2; id <= 256; id += 1) {
      const chunks: string[] = []
      for (let index = 0; index < 8; index += 1) {
        const bytes = crypto.getRandomValues(new Uint8Array(64 * 1_024))
        chunks.push(btoa(String.fromCharCode(...bytes)))
      }
      try {
        await store.commitPage('U1', page([{
          id,
          conversation: 'direct:U1:U2',
          content: chunks.join(''),
        }], id))
        lastCommittedSeq = id
      } catch (error) {
        const candidate = error as Error
        store.close()
        return {
          errorName: candidate.name,
          errorMessage: candidate.message,
          classified: isLocalSyncCapacityError(candidate),
          lastCommittedSeq,
          rejectedSeq: id,
        }
      }
    }
    store.close()
    throw new Error('constrained profile accepted every IndexedDB page')
  },

  async cleanup(databaseName: string) {
    await deleteDatabase(databaseName)
  },

  classifyNativeQuota() {
    const error = new DOMException('browser quota exceeded', 'QuotaExceededError')
    return { name: error.name, message: error.message, classified: isLocalSyncCapacityError(error) }
  },
}

declare global {
  interface Window {
    dipoleIndexedDBAcceptance: typeof acceptance
  }
}

window.dipoleIndexedDBAcceptance = acceptance
