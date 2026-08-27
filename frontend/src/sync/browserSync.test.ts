import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('@/api', () => ({ default: { get, post } }))

import { createBrowserTimelineNotifyVerifier, reportBrowserSyncFailure, timelineNotificationPath } from './browserSync'

describe('browser Sync failure telemetry', () => {
  beforeEach(() => post.mockReset().mockResolvedValue({}))

  it('reports native quota failures as a bounded storage_full observation', async () => {
    await reportBrowserSyncFailure(new DOMException('browser quota exceeded', 'QuotaExceededError'))

    expect(post).toHaveBeenCalledWith('/api/v1/sync/comparison', {
      baseline: false,
      match: 0,
      pending: 0,
      legacy_only: 0,
      sync_only: 0,
      overflow: 0,
      storage_full: 1,
      sync_error: 0,
    })
  })

  it('reports ordinary recovery failures without propagating telemetry errors', async () => {
    post.mockRejectedValueOnce(new Error('offline'))
    await expect(reportBrowserSyncFailure(new Error('sync failed'))).resolves.toBeUndefined()
    expect(post).toHaveBeenCalledWith('/api/v1/sync/comparison', expect.objectContaining({
      storage_full: 0,
      sync_error: 1,
    }))
  })
})

describe('browser Timeline notification shadow adapter', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset().mockResolvedValue({})
  })

  it('pulls a direct Timeline by peer and reports only a bounded outcome', async () => {
    get.mockResolvedValueOnce([{ message_id: 'M42', message_seq: 42 }])
    const verifier = createBrowserTimelineNotifyVerifier('U2')

    await verifier.observe({
      schema_version: 'v1', event_id: 'E42', message_uuid: 'M42',
      conversation_key: 'direct:U1:U2', message_seq: 42, target_type: 0, target_uuid: 'U2',
    })

    expect(get).toHaveBeenCalledWith('/api/v1/messages/direct/U1?after_seq=41&limit=100')
    expect(post).toHaveBeenCalledWith('/api/v1/sync/comparison', {
      timeline_match: 1,
      timeline_missing: 0,
      timeline_mismatch: 0,
      timeline_error: 0,
      timeline_invalid: 0,
    })
  })

  it('builds a group path and rejects a forged direct recipient', () => {
    expect(timelineNotificationPath('U2', {
      schema_version: 'v1', event_id: 'EG', message_uuid: 'MG',
      conversation_key: 'group:G1', message_seq: 8, target_type: 1, target_uuid: 'G1',
    }, 7, 20)).toBe('/api/v1/messages/group/G1?after_seq=7&limit=20')

    expect(() => timelineNotificationPath('U3', {
      schema_version: 'v1', event_id: 'E42', message_uuid: 'M42',
      conversation_key: 'direct:U1:U2', message_seq: 42, target_type: 0, target_uuid: 'U2',
    }, 41, 100)).toThrow('locator is invalid')
  })
})
