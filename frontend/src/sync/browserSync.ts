import api from '@/api'
import type { Message } from '@/types'
import { IndexedDBSyncStore } from './indexedDBSyncStore'
import { MessageSyncEngine, type SyncPage } from './syncEngine'

export const browserSyncEnabled = import.meta.env.VITE_SYNC_ENGINE_ENABLED === 'true'

let store: IndexedDBSyncStore | undefined

export async function recoverBrowserMessages(userUUID: string, deliver: (messages: Message[]) => void) {
  const engine = new MessageSyncEngine(getStore(), {
    list: (afterSeq, limit) => api.get(`/api/v1/sync?after_seq=${afterSeq}&limit=${limit}`) as Promise<SyncPage>,
    acknowledge: async syncSeq => {
      await api.patch('/api/v1/sync/checkpoint', { sync_seq: syncSeq })
    },
  })
  return engine.recover(userUUID, deliver)
}

export async function clearBrowserMessages(userUUID: string) {
  if (!userUUID || typeof indexedDB === 'undefined') return
  await getStore().clearUser(userUUID)
}

function getStore() {
  if (typeof indexedDB === 'undefined') throw new Error('IndexedDB is unavailable')
  store ??= new IndexedDBSyncStore()
  return store
}
