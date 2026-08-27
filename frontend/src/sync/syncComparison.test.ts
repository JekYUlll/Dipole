import { describe, expect, it } from 'vitest'
import type { Message } from '@/types'
import {
  compareSyncMessages,
  emptySyncComparisonState,
  incomingDirectMessageIDs,
  normalizeSyncComparisonState,
} from './syncComparison'

describe('compareSyncMessages', () => {
  it('re-baselines corrupt persisted comparison state', () => {
    expect(normalizeSyncComparisonState({ version: 1, ready: true, baselineMessageIDs: [], pending: { M1: null } }))
      .toEqual(emptySyncComparisonState())
  })

  it('compares only incoming direct messages shared by both protocol semantics', () => {
    const value = (id: string, from: string, target: string, targetType = 0): Message => ({
      id: 1, message_id: id, message_seq: 1, from_uuid: from, target_uuid: target,
      target_type: targetType, message_type: 0, content: id, sent_at: '2026-08-27T12:00:00Z',
    })

    expect(incomingDirectMessageIDs('U1', [
      value('incoming', 'U2', 'U1'),
      value('outgoing', 'U1', 'U2'),
      value('group', 'U2', 'G1', 1),
    ])).toEqual(['incoming'])
  })

  it('establishes a bounded first-run baseline without reporting historical differences', () => {
    const result = compareSyncMessages(emptySyncComparisonState(), {
      legacyMessageIDs: ['M1', 'M2'],
      syncMessageIDs: ['M2', 'M3'],
      now: 1_000,
    })

    expect(result.report).toEqual({ baseline: true, match: 0, pending: 0, legacyOnly: 0, syncOnly: 0, overflow: 0 })
    expect(result.state.baselineMessageIDs).toEqual(['M1', 'M2', 'M3'])
  })

  it('matches a new stable message UUID observed by both protocols', () => {
    const baseline = compareSyncMessages(emptySyncComparisonState(), {
      legacyMessageIDs: ['OLD'], syncMessageIDs: ['OLD'], now: 1_000,
    }).state

    const result = compareSyncMessages(baseline, {
      legacyMessageIDs: ['M4'], syncMessageIDs: ['M4'], now: 2_000,
    })

    expect(result.report.match).toBe(1)
    expect(result.report.pending).toBe(0)
    expect(result.state.pending).toEqual({})
  })

  it('keeps one-sided observations pending through the grace window', () => {
    const state = compareSyncMessages(emptySyncComparisonState(), {
      legacyMessageIDs: [], syncMessageIDs: [], now: 1_000,
    }).state

    const pending = compareSyncMessages(state, {
      legacyMessageIDs: ['M5'], syncMessageIDs: [], now: 2_000,
    }, { graceMS: 60_000 })
    expect(pending.report).toEqual(expect.objectContaining({ pending: 1, legacyOnly: 0, syncOnly: 0 }))

    const matched = compareSyncMessages(pending.state, {
      legacyMessageIDs: [], syncMessageIDs: ['M5'], now: 50_000,
    }, { graceMS: 60_000 })
    expect(matched.report).toEqual(expect.objectContaining({ match: 1, pending: 0 }))
  })

  it('reports a one-sided observation only after its grace period expires', () => {
    const state = compareSyncMessages(emptySyncComparisonState(), {
      legacyMessageIDs: [], syncMessageIDs: [], now: 1_000,
    }).state
    const pending = compareSyncMessages(state, {
      legacyMessageIDs: [], syncMessageIDs: ['M6'], now: 2_000,
    }, { graceMS: 10_000 })

    const expired = compareSyncMessages(pending.state, {
      legacyMessageIDs: [], syncMessageIDs: [], now: 12_001,
    }, { graceMS: 10_000 })

    expect(expired.report).toEqual(expect.objectContaining({ pending: 0, legacyOnly: 0, syncOnly: 1 }))
    expect(expired.state.pending).toEqual({})
  })

  it('bounds pending state and reports dropped observations', () => {
    const state = compareSyncMessages(emptySyncComparisonState(), {
      legacyMessageIDs: [], syncMessageIDs: [], now: 1_000,
    }).state
    const result = compareSyncMessages(state, {
      legacyMessageIDs: ['M1', 'M2', 'M3'], syncMessageIDs: [], now: 2_000,
    }, { maxPending: 2, graceMS: 60_000 })

    expect(result.report).toEqual(expect.objectContaining({ pending: 2, overflow: 1 }))
    expect(Object.keys(result.state.pending)).toEqual(['M2', 'M3'])
  })
})
