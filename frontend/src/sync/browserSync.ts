import api from '@/api'
import type { Message } from '@/types'
import { GroupMessageSyncEngine, type GroupSyncTarget } from './groupSyncEngine'
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

export async function recoverBrowserGroupMessages(
  userUUID: string,
  target: GroupSyncTarget,
  deliver: (messages: Message[], source: SyncDeliverySource) => void,
) {
  const engine = new GroupMessageSyncEngine(getStore(), {
    list: async (groupUUID, afterSeq, limit) => {
      const messages = await api.get(`/api/v1/messages/group/${groupUUID}?after_seq=${afterSeq}&limit=${limit}`)
      return Array.isArray(messages) ? messages as Message[] : []
    },
    acknowledge: async (groupUUID, messageSeq) => {
      await api.patch(`/api/v1/sync/groups/${groupUUID}/checkpoint`, { message_seq: messageSeq })
    },
  })
  return engine.recover(userUUID, target, deliver)
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

export async function reportBrowserSyncFailure(error: unknown) {
  const storageFull = isLocalSyncCapacityError(error)
  try {
    await api.post('/api/v1/sync/comparison', {
      baseline: false,
      match: 0,
      pending: 0,
      legacy_only: 0,
      sync_only: 0,
      overflow: 0,
      storage_full: storageFull ? 1 : 0,
      sync_error: storageFull ? 0 : 1,
    })
  } catch {
    // Client telemetry cannot delay recovery or replace the original Sync error.
  }
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
