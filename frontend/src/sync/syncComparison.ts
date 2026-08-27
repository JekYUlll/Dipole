import type { Message } from '@/types'

export interface PendingSyncObservation {
  firstSeenAt: number
  legacy: boolean
  sync: boolean
}

export interface SyncComparisonState {
  version: 1
  ready: boolean
  baselineMessageIDs: string[]
  pending: Record<string, PendingSyncObservation>
}

export interface SyncComparisonReport {
  baseline: boolean
  match: number
  pending: number
  legacyOnly: number
  syncOnly: number
  overflow: number
}

export const emptySyncComparisonState = (): SyncComparisonState => ({
  version: 1,
  ready: false,
  baselineMessageIDs: [],
  pending: {},
})

export function normalizeSyncComparisonState(value: unknown): SyncComparisonState {
  if (!value || typeof value !== 'object') return emptySyncComparisonState()
  const state = value as Partial<SyncComparisonState>
  if (state.version !== 1 || typeof state.ready !== 'boolean' || !Array.isArray(state.baselineMessageIDs) || !state.pending || typeof state.pending !== 'object') {
    return emptySyncComparisonState()
  }
  if (state.baselineMessageIDs.some(messageID => typeof messageID !== 'string')) return emptySyncComparisonState()
  for (const observation of Object.values(state.pending)) {
    if (!observation || !Number.isFinite(observation.firstSeenAt) || typeof observation.legacy !== 'boolean' || typeof observation.sync !== 'boolean') {
      return emptySyncComparisonState()
    }
  }
  return {
    version: 1,
    ready: state.ready,
    baselineMessageIDs: uniqueIDs(state.baselineMessageIDs).slice(-512),
    pending: Object.fromEntries(sortedPending(state.pending).slice(-512)),
  }
}

export function incomingDirectMessageIDs(userUUID: string, messages: Message[]) {
  return uniqueIDs(messages
    .filter(message => message.target_type === 0 && message.target_uuid === userUUID && message.from_uuid !== userUUID)
    .map(message => message.message_id))
}

export function compareSyncMessages(
  current: SyncComparisonState,
  sample: { legacyMessageIDs: string[]; syncMessageIDs: string[]; now: number },
  options: { graceMS?: number; maxPending?: number; maxBaseline?: number } = {},
) {
  const graceMS = Math.max(0, options.graceMS ?? 60_000)
  const maxPending = Math.max(1, options.maxPending ?? 512)
  const maxBaseline = Math.max(1, options.maxBaseline ?? 512)
  const legacyMessageIDs = uniqueIDs(sample.legacyMessageIDs)
  const syncMessageIDs = uniqueIDs(sample.syncMessageIDs)

  if (!current.ready) {
    const baselineMessageIDs = uniqueIDs([...legacyMessageIDs, ...syncMessageIDs]).slice(-maxBaseline)
    return {
      state: { version: 1, ready: true, baselineMessageIDs, pending: {} } satisfies SyncComparisonState,
      report: report(true),
    }
  }

  const baseline = new Set(current.baselineMessageIDs)
  const pending = Object.fromEntries(
    Object.entries(current.pending).map(([messageID, observation]) => [messageID, { ...observation }]),
  )
  observe(pending, baseline, legacyMessageIDs, 'legacy', sample.now)
  observe(pending, baseline, syncMessageIDs, 'sync', sample.now)

  const result = report(false)
  for (const [messageID, observation] of sortedPending(pending)) {
    if (observation.legacy && observation.sync) {
      result.match += 1
      delete pending[messageID]
      continue
    }
    if (sample.now - observation.firstSeenAt < graceMS) continue
    if (observation.legacy) result.legacyOnly += 1
    if (observation.sync) result.syncOnly += 1
    delete pending[messageID]
  }

  const overflow = Math.max(0, Object.keys(pending).length - maxPending)
  for (const [messageID] of sortedPending(pending).slice(0, overflow)) delete pending[messageID]
  result.overflow = overflow
  result.pending = Object.keys(pending).length

  return {
    state: { ...current, pending },
    report: result,
  }
}

function observe(
  pending: Record<string, PendingSyncObservation>,
  baseline: Set<string>,
  messageIDs: string[],
  source: 'legacy' | 'sync',
  now: number,
) {
  for (const messageID of messageIDs) {
    if (baseline.has(messageID)) continue
    const observation = pending[messageID] ?? { firstSeenAt: now, legacy: false, sync: false }
    observation[source] = true
    pending[messageID] = observation
  }
}

function uniqueIDs(messageIDs: string[]) {
  return [...new Set(messageIDs.map(messageID => messageID.trim()).filter(Boolean))]
}

function sortedPending(pending: Record<string, PendingSyncObservation>) {
  return Object.entries(pending).sort((left, right) =>
    left[1].firstSeenAt - right[1].firstSeenAt || left[0].localeCompare(right[0]),
  )
}

function report(baseline: boolean): SyncComparisonReport {
  return { baseline, match: 0, pending: 0, legacyOnly: 0, syncOnly: 0, overflow: 0 }
}
