export type MultipartLockManager = {
  request<T>(
    name: string,
    options: { mode: 'exclusive' },
    callback: (lock: unknown) => Promise<T>,
  ): Promise<T>
}

const browserLocks = (): MultipartLockManager | undefined => {
  if (typeof navigator === 'undefined') return undefined
  return (navigator as Navigator & { locks?: MultipartLockManager }).locks
}

// Web Locks serializes same-file uploads across tabs without introducing a
// second server-side lease that could outlive a crashed browser tab.
export const withMultipartUploadLease = async <T>(
  name: string,
  task: () => Promise<T>,
  locks: MultipartLockManager | undefined = browserLocks(),
): Promise<T> => {
  if (!locks) return task()
  return locks.request(name, { mode: 'exclusive' }, async () => task())
}
