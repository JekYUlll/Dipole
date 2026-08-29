import { describe, expect, it, vi } from 'vitest'
import { uploadMultipartParts } from './multipartUpload'

describe('uploadMultipartParts', () => {
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
    expect(progress).toHaveLength(5)
    expect(progress.every(([, total]) => total === 5)).toBe(true)
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
