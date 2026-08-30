export type MultipartUploadOptions = {
  concurrency?: number
  maxRetries?: number
  retryDelayMs?: number
  sleep?: (delayMs: number) => Promise<void>
  skipParts?: ReadonlySet<number>
  onPartComplete?: (completedParts: number, totalParts: number) => void
  isPaused?: () => boolean
  waitUntilResumed?: () => Promise<void>
}

type UploadPart = (partNumber: number, chunk: Blob) => Promise<void>

export type PresignedPartFetch = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>

export class PresignedPartUploadError extends Error {
  constructor(public readonly status: number) {
    super(`presigned part upload failed with status ${status}`)
    this.name = 'PresignedPartUploadError'
  }
}

export const toSameOriginPresignedURL = (
  value: string,
  enabled: boolean,
  origin = globalThis.location?.origin,
): string => {
  if (!enabled || !origin) return value
  try {
    const target = new URL(value)
    const sameOrigin = new URL(origin)
    target.protocol = sameOrigin.protocol
    target.hostname = sameOrigin.hostname
    target.port = sameOrigin.port
    return target.toString()
  } catch {
    return value
  }
}

const defaultSleep = (delayMs: number) => new Promise<void>(resolve => setTimeout(resolve, delayMs))

export const uploadPresignedPart = async (
  url: string,
  chunk: Blob,
  fetchImpl: PresignedPartFetch = globalThis.fetch.bind(globalThis),
): Promise<string> => {
  const response = await fetchImpl(url, {
    method: 'PUT',
    body: chunk,
  })
  if (!response.ok) throw new PresignedPartUploadError(response.status)
  const etag = response.headers.get('ETag')?.trim()
  if (!etag) throw new Error('presigned part upload did not return an ETag')
  return etag
}

export const sha256Hex = async (chunk: Blob): Promise<string | undefined> => {
  if (!globalThis.crypto?.subtle || typeof chunk.arrayBuffer !== 'function') return undefined
  const digest = await globalThis.crypto.subtle.digest('SHA-256', await chunk.arrayBuffer())
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('')
}

const uploadWithRetry = async (
  partNumber: number,
  chunk: Blob,
  uploadPart: UploadPart,
  maxRetries: number,
  retryDelayMs: number,
  sleep: (delayMs: number) => Promise<void>,
) => {
  for (let attempt = 0; ; attempt += 1) {
    try {
      await uploadPart(partNumber, chunk)
      return
    } catch (error) {
      if (attempt >= maxRetries) throw error
      await sleep(retryDelayMs * 2 ** attempt)
    }
  }
}

export const uploadMultipartParts = async (
  file: Blob,
  chunkSize: number,
  totalParts: number,
  uploadPart: UploadPart,
  options: MultipartUploadOptions = {},
) => {
  if (file.size <= 0 || chunkSize <= 0 || totalParts <= 0) {
    throw new Error('invalid multipart upload dimensions')
  }

  const expectedParts = Math.ceil(file.size / chunkSize)
  if (expectedParts !== totalParts) {
    throw new Error('multipart upload part count does not match file size')
  }

  const concurrency = Math.max(1, Math.min(Math.floor(options.concurrency ?? 3), totalParts))
  const maxRetries = Math.max(0, Math.floor(options.maxRetries ?? 2))
  const retryDelayMs = Math.max(0, options.retryDelayMs ?? 250)
  const sleep = options.sleep ?? defaultSleep
  const skipParts = options.skipParts ?? new Set<number>()
  let nextPart = 1
  let completedParts = skipParts.size
  let stopped = false
  let firstError: unknown

  options.onPartComplete?.(completedParts, totalParts)

  const worker = async () => {
    while (!stopped) {
      if (options.isPaused?.()) {
        if (options.waitUntilResumed === undefined) throw new Error('paused multipart upload requires a resume handler')
        await options.waitUntilResumed()
        continue
      }
      const partNumber = nextPart++
      if (partNumber > totalParts) return

      if (skipParts.has(partNumber)) continue

      const start = (partNumber - 1) * chunkSize
      const chunk = file.slice(start, Math.min(start + chunkSize, file.size))
      try {
        await uploadWithRetry(partNumber, chunk, uploadPart, maxRetries, retryDelayMs, sleep)
        completedParts += 1
        options.onPartComplete?.(completedParts, totalParts)
      } catch (error) {
        stopped = true
        firstError ??= error
        return
      }
    }
  }

  await Promise.all(Array.from({ length: concurrency }, worker))
  if (firstError) throw firstError
}
