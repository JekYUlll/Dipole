import { describe, expect, it, vi } from 'vitest'
import { sha256Hex, toSameOriginPresignedURL, uploadMultipartParts, uploadPresignedPart } from './multipartUpload'

describe('same-origin presigned upload URL', () => {
  it('rewrites only the origin when the Gateway proxy is enabled', () => {
    expect(toSameOriginPresignedURL(
      'http://minio:9000/dipole-files/object?partNumber=1&X-Amz-Signature=sig',
      true,
      'https://chat.example.test',
    )).toBe('https://chat.example.test/dipole-files/object?partNumber=1&X-Amz-Signature=sig')
  })

  it('keeps the storage URL when the proxy is disabled', () => {
    const value = 'http://minio:9000/dipole-files/object?partNumber=1'
    expect(toSameOriginPresignedURL(value, false, 'https://chat.example.test')).toBe(value)
  })
})

describe('uploadMultipartParts', () => {
  it('uploads a presigned part without sending it through the application API', async () => {
    const fetchImpl = vi.fn(async () => new Response(null, {
      status: 200,
      headers: { ETag: '"etag-1"' },
    }))

    const etag = await uploadPresignedPart('https://minio.test/part-1', new Blob(['data']), fetchImpl)

    expect(etag).toBe('"etag-1"')
    expect(fetchImpl).toHaveBeenCalledWith('https://minio.test/part-1', expect.objectContaining({ method: 'PUT' }))
  })

  it('rejects a successful direct upload without an ETag', async () => {
    const fetchImpl = vi.fn(async () => new Response(null, { status: 200 }))
    await expect(uploadPresignedPart('https://minio.test/part-1', new Blob(['data']), fetchImpl)).rejects.toThrow('ETag')
  })

  it('preserves the HTTP status when a presigned URL expires', async () => {
    const fetchImpl = vi.fn(async () => new Response(null, { status: 403 }))
    await expect(uploadPresignedPart('https://minio.test/part-1', new Blob(['data']), fetchImpl))
      .rejects.toMatchObject({ name: 'PresignedPartUploadError', status: 403 })
  })

  it('computes a stable SHA-256 checksum when Web Crypto is available', async () => {
    const checksum = await sha256Hex(new Blob(['data']))
    if (checksum === undefined) return
    expect(checksum).toBe('3a6eb0790f39ac87c94f3856b2dd2c5d110e6811602261a9a923d3bb23adc8b7')
  })

  it('uploads parts with bounded concurrency and reports completion', async () => {
    const file = new Blob(['0123456789'])
    const uploaded: Array<{ partNumber: number; size: number }> = []
    const progress: Array<[number, number]> = []
    let active = 0
    let maximumActive = 0

    await uploadMultipartParts(file, 2, 5, async (partNumber, chunk) => {
      active += 1
      maximumActive = Math.max(maximumActive, active)
      await new Promise(resolve => setTimeout(resolve, 1))
      uploaded.push({ partNumber, size: chunk.size })
      active -= 1
    }, {
      concurrency: 2,
      onPartComplete: (completed, total) => progress.push([completed, total]),
    })

    expect(maximumActive).toBe(2)
    expect(uploaded.sort((a, b) => a.partNumber - b.partNumber)).toEqual([
      { partNumber: 1, size: 2 },
      { partNumber: 2, size: 2 },
      { partNumber: 3, size: 2 },
      { partNumber: 4, size: 2 },
      { partNumber: 5, size: 2 },
    ])
    expect(progress).toHaveLength(6)
    expect(progress[0]).toEqual([0, 5])
    expect(progress.every(([, total]) => total === 5)).toBe(true)
  })

  it('skips parts already confirmed by the server when resuming', async () => {
    const file = new Blob(['0123456789'])
    const uploaded: number[] = []
    const progress: Array<[number, number]> = []

    await uploadMultipartParts(file, 2, 5, async partNumber => {
      uploaded.push(partNumber)
    }, {
      concurrency: 2,
      skipParts: new Set([1, 3]),
      onPartComplete: (completed, total) => progress.push([completed, total]),
    })

    expect(uploaded.sort((a, b) => a - b)).toEqual([2, 4, 5])
    expect(progress[0]).toEqual([2, 5])
    expect(progress.at(-1)).toEqual([5, 5])
  })

  it('pauses scheduling without discarding the resumable session', async () => {
    const file = new Blob(['0123456789'])
    const uploaded: number[] = []
    let paused = true
    let releaseResume!: () => void
    const resumed = new Promise<void>(resolve => { releaseResume = resolve })
    const run = uploadMultipartParts(file, 2, 5, async partNumber => {
      uploaded.push(partNumber)
    }, {
      concurrency: 2,
      isPaused: () => paused,
      waitUntilResumed: () => resumed,
    })

    await new Promise(resolve => setTimeout(resolve, 0))
    expect(uploaded).toEqual([])
    paused = false
    releaseResume()
    await run
    expect(uploaded).toHaveLength(5)
  })

  it('retries a failed part with exponential backoff', async () => {
    const file = new Blob(['abcd'])
    const attempts = vi.fn()
    const sleeps: number[] = []

    await uploadMultipartParts(file, 4, 1, async () => {
      attempts()
      if (attempts.mock.calls.length < 3) throw new Error('temporary failure')
    }, {
      maxRetries: 2,
      retryDelayMs: 10,
      sleep: async delayMs => { sleeps.push(delayMs) },
    })

    expect(attempts).toHaveBeenCalledTimes(3)
    expect(sleeps).toEqual([10, 20])
  })

  it('stops scheduling new parts after a permanent failure', async () => {
    const file = new Blob(['0123456789'])
    const attempted: number[] = []

    await expect(uploadMultipartParts(file, 2, 5, async partNumber => {
      attempted.push(partNumber)
      if (partNumber === 1) throw new Error('permanent failure')
    }, { concurrency: 1, maxRetries: 0 })).rejects.toThrow('permanent failure')

    expect(attempted).toEqual([1])
  })
})
