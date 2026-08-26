import api from '@/api'
import type { Message } from '@/types'
import { IndexedDBSyncStore, isLocalSyncCapacityError } from './indexedDBSyncStore'
import {
  compareSyncMessages,
  emptySyncComparisonState,
  incomingDirectMessageIDs,
  normalizeSyncComparisonState,
  type SyncComparisonState,
} from './syncComparison'
import { MessageSyncEngine, type SyncDeliverySource, type SyncPage } from './syncEngine'

export type BrowserSyncMode = 'off' | 'shadow' | 'primary'

const configuredMode = import.meta.env.VITE_SYNC_ENGINE_MODE
export const browserSyncMode: BrowserSyncMode = configuredMode === 'shadow' || configuredMode === 'primary'
  ? configuredMode
  : import.meta.env.VITE_SYNC_ENGINE_ENABLED === 'true' ? 'primary' : 'off'
export const browserSyncEnabled = browserSyncMode !== 'off'
export { isLocalSyncCapacityError }

let store: IndexedDBSyncStore | undefined

export async function recoverBrowserMessages(
  userUUID: string,
  deliver: (messages: Message[], source: SyncDeliverySource) => void,
) {
  const engine = new MessageSyncEngine(getStore(), {
    list: (afterSeq, limit) => api.get(`/api/v1/sync?after_seq=${afterSeq}&limit=${limit}`) as Promise<SyncPage>,
    acknowledge: async syncSeq => {
      await api.patch('/api/v1/sync/checkpoint', { sync_seq: syncSeq })
    },
  })
  return engine.recover(userUUID, deliver)
}

export async function compareBrowserSyncMessages(userUUID: string, legacyMessages: Message[], syncMessages: Message[]) {
  const key = comparisonStorageKey(userUUID)
  const current = readComparisonState(key)
  const result = compareSyncMessages(current, {
    legacyMessageIDs: incomingDirectMessageIDs(userUUID, legacyMessages),
    syncMessageIDs: incomingDirectMessageIDs(userUUID, syncMessages),
    now: Date.now(),
  })
  try {
    localStorage.setItem(key, JSON.stringify(result.state))
  } catch {
    // Browser quota or privacy mode cannot break either synchronization path.
  }
  try {
    await api.post('/api/v1/sync/comparison', {
      baseline: result.report.baseline,
      match: result.report.match,
      pending: result.report.pending,
      legacy_only: result.report.legacyOnly,
      sync_only: result.report.syncOnly,
      overflow: result.report.overflow,
    })
  } catch {
    // Comparison telemetry cannot block either client synchronization path.
  }
  return result.report
}

export async function clearBrowserMessages(userUUID: string) {
  if (!userUUID) return
  localStorage.removeItem(comparisonStorageKey(userUUID))
  if (typeof indexedDB !== 'undefined') await getStore().clearUser(userUUID)
}

function getStore() {
  if (typeof indexedDB === 'undefined') throw new Error('IndexedDB is unavailable')
  store ??= new IndexedDBSyncStore()
  return store
}

function comparisonStorageKey(userUUID: string) {
  return `dipole.web.syncComparison.${userUUID}`
}

function readComparisonState(key: string): SyncComparisonState {
  try {
    return normalizeSyncComparisonState(JSON.parse(localStorage.getItem(key) || ''))
  } catch {
    // Corrupt local telemetry state is safe to re-baseline.
  }
  return emptySyncComparisonState()
}
