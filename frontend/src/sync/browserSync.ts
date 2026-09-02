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
import {
  TimelineNotifyShadowVerifier,
  type TimelineNotification,
  type TimelineNotifyShadowOutcome,
} from './timelineNotifyShadow'

export type BrowserSyncMode = 'off' | 'shadow' | 'primary'

const configuredMode = import.meta.env.VITE_SYNC_ENGINE_MODE
export const browserSyncMode: BrowserSyncMode = configuredMode === 'shadow' || configuredMode === 'primary'
  ? configuredMode
  : import.meta.env.VITE_SYNC_ENGINE_ENABLED === 'true' ? 'primary' : 'off'
export const browserSyncEnabled = browserSyncMode !== 'off'
export type TimelineNotifyMode = 'off' | 'shadow' | 'primary'
const configuredTimelineNotifyMode = import.meta.env.VITE_TIMELINE_NOTIFY_MODE
export const timelineNotifyMode: TimelineNotifyMode = configuredTimelineNotifyMode === 'shadow' || configuredTimelineNotifyMode === 'primary'
  ? configuredTimelineNotifyMode
  : 'off'
export const timelineNotifyShadowEnabled = timelineNotifyMode === 'shadow'
export const timelineNotifyPrimaryEnabled = timelineNotifyMode === 'primary'
export { isLocalSyncCapacityError }

let store: IndexedDBSyncStore | undefined
let timelineVerifier: { userUUID: string; verifier: TimelineNotifyShadowVerifier } | undefined

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

export function createBrowserTimelineNotifyVerifier(userUUID: string, deliver?: (messages: Message[]) => void) {
  return new TimelineNotifyShadowVerifier({
    list: async (notification, afterSeq, limit) => {
      const path = timelineNotificationPath(userUUID, notification, afterSeq, limit)
      const messages = await api.get(path)
      return Array.isArray(messages) ? messages as Message[] : []
    },
  }, reportTimelineNotifyShadowOutcome, timelineNotifyPrimaryEnabled ? deliver : undefined)
}

export function observeBrowserTimelineNotification(userUUID: string, notification: unknown, deliver?: (messages: Message[]) => void) {
  if (timelineNotifyMode === 'off' || !userUUID) return Promise.resolve()
  if (!timelineVerifier || timelineVerifier.userUUID !== userUUID) {
    timelineVerifier = { userUUID, verifier: createBrowserTimelineNotifyVerifier(userUUID, deliver) }
  }
  return timelineVerifier.verifier.observe(notification)
}

export function timelineNotificationPath(
  userUUID: string,
  notification: TimelineNotification,
  afterSeq: number,
  limit: number,
) {
  if (notification.target_type === 1) {
    if (notification.conversation_key !== `group:${notification.target_uuid}`) {
      throw new Error('group timeline notification locator is invalid')
    }
    return `/api/v1/messages/group/${encodeURIComponent(notification.target_uuid)}?after_seq=${afterSeq}&limit=${limit}`
  }
  const parts = notification.conversation_key.split(':')
  if (parts.length !== 3 || parts[0] !== 'direct' || notification.target_uuid !== userUUID) {
    throw new Error('direct timeline notification locator is invalid')
  }
  const participants = [parts[1], parts[2]]
  if (!participants.includes(userUUID) || participants[0] === participants[1]) {
    throw new Error('direct timeline notification participants are invalid')
  }
  const peerUUID = participants[0] === userUUID ? participants[1] : participants[0]
  return `/api/v1/messages/direct/${encodeURIComponent(peerUUID)}?after_seq=${afterSeq}&limit=${limit}`
}

async function reportTimelineNotifyShadowOutcome(outcome: TimelineNotifyShadowOutcome) {
  const counts = {
    timeline_match: 0,
    timeline_missing: 0,
    timeline_mismatch: 0,
    timeline_error: 0,
    timeline_invalid: 0,
  }
  counts[`timeline_${outcome}` as keyof typeof counts] = 1
  try {
    await api.post('/api/v1/sync/comparison', counts)
  } catch {
    // Shadow telemetry cannot affect realtime delivery or Timeline verification.
  }
}

export async function clearBrowserMessages(userUUID: string) {
  if (!userUUID) return
  localStorage.removeItem(comparisonStorageKey(userUUID))
  if (timelineVerifier?.userUUID === userUUID) timelineVerifier = undefined
  if (typeof indexedDB !== 'undefined') await getStore().clearUser(userUUID)
}

export async function claimBrowserDelivery(userUUID: string, deliveryID: string) {
  return getStore().claimDelivery(userUUID, deliveryID)
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
