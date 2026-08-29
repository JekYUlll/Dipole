export type MultipartUploadOptions = {
  concurrency?: number
  maxRetries?: number
  retryDelayMs?: number
  sleep?: (delayMs: number) => Promise<void>
  onPartComplete?: (completedParts: number, totalParts: number) => void
}

type UploadPart = (partNumber: number, chunk: Blob) => Promise<void>

const defaultSleep = (delayMs: number) => new Promise<void>(resolve => setTimeout(resolve, delayMs))

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
  let nextPart = 1
  let completedParts = 0
  let stopped = false
  let firstError: unknown

  const worker = async () => {
    while (!stopped) {
      const partNumber = nextPart++
      if (partNumber > totalParts) return

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
