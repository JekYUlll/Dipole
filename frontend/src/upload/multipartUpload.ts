export type MultipartUploadOptions = {
  concurrency?: number
  maxRetries?: number
  retryDelayMs?: number
  sleep?: (delayMs: number) => Promise<void>
  skipParts?: ReadonlySet<number>
  signal?: AbortSignal
  onPartComplete?: (completedParts: number, totalParts: number) => void
  isPaused?: () => boolean
  waitUntilResumed?: () => Promise<void>
}

type UploadPart = (partNumber: number, chunk: Blob, signal?: AbortSignal) => Promise<void>

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
  signal?: AbortSignal,
): Promise<string> => {
  const response = await fetchImpl(url, {
    method: 'PUT',
    body: chunk,
    signal,
  })
  if (!response.ok) throw new PresignedPartUploadError(response.status)
  const etag = response.headers.get('ETag')?.trim()
  if (!etag) throw new Error('presigned part upload did not return an ETag')
  return etag
}

export const uploadPresignedPartWithRefresh = async (
  url: string,
  chunk: Blob,
  refreshURL: () => Promise<string>,
  fetchImpl: PresignedPartFetch = globalThis.fetch.bind(globalThis),
  signal?: AbortSignal,
): Promise<string> => {
  try {
    return await uploadPresignedPart(url, chunk, fetchImpl, signal)
  } catch (error) {
    if (!(error instanceof PresignedPartUploadError) || ![401, 403].includes(error.status)) throw error
    if (signal?.aborted) throw signal.reason ?? createAbortError()
    return uploadPresignedPart(await refreshURL(), chunk, fetchImpl, signal)
  }
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
  signal?: AbortSignal,
) => {
  for (let attempt = 0; ; attempt += 1) {
    throwIfAborted(signal)
    try {
      await uploadPart(partNumber, chunk, signal)
      return
    } catch (error) {
      if (attempt >= maxRetries || !isRetryableMultipartUploadError(error)) throw error
      throwIfAborted(signal)
      await waitWithAbort(sleep(retryDelayMs * 2 ** attempt), signal)
    }
  }
}

// Network failures do not expose an HTTP status and can be transient. Once a
// presigned PUT has a status, retry only states that can plausibly recover.
function isRetryableMultipartUploadError(error: unknown): boolean {
  if (!(error instanceof PresignedPartUploadError)) return true
  return error.status === 408 || error.status === 429 || error.status >= 500
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
  throwIfAborted(options.signal)
  let nextPart = 1
  let completedParts = skipParts.size
  let stopped = false
  let firstError: unknown

  options.onPartComplete?.(completedParts, totalParts)

  const worker = async () => {
    while (!stopped) {
      if (options.isPaused?.()) {
        if (options.waitUntilResumed === undefined) throw new Error('paused multipart upload requires a resume handler')
        await waitWithAbort(options.waitUntilResumed(), options.signal)
        throwIfAborted(options.signal)
        continue
      }
      const partNumber = nextPart++
      if (partNumber > totalParts) return

      if (skipParts.has(partNumber)) continue

      const start = (partNumber - 1) * chunkSize
      const chunk = file.slice(start, Math.min(start + chunkSize, file.size))
      try {
        await uploadWithRetry(partNumber, chunk, uploadPart, maxRetries, retryDelayMs, sleep, options.signal)
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

function throwIfAborted(signal?: AbortSignal): void {
  if (signal?.aborted) throw signal.reason ?? createAbortError()
}

function createAbortError(): Error {
  return typeof DOMException === 'function'
    ? new DOMException('The operation was aborted', 'AbortError')
    : new Error('The operation was aborted')
}

async function waitWithAbort<T>(pending: Promise<T>, signal?: AbortSignal): Promise<T> {
  if (!signal) return pending
  throwIfAborted(signal)
  return new Promise<T>((resolve, reject) => {
    const cleanup = () => signal.removeEventListener('abort', onAbort)
    const onAbort = () => {
      cleanup()
      reject(signal.reason ?? createAbortError())
    }
    signal.addEventListener('abort', onAbort, { once: true })
    pending.then(
      (value) => { cleanup(); resolve(value) },
      (error) => { cleanup(); reject(error) },
    )
  })
}
