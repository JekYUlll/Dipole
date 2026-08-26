import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))

vi.mock('@/api', () => ({ default: { post } }))

import { reportBrowserSyncFailure } from './browserSync'

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
