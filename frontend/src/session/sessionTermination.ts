export interface SessionStorage {
  getItem(key: string): string | null
  removeItem(key: string): void
}

type RuntimeClear = () => void
type UserDataClear = (userUUID: string) => Promise<void>
type LoginRedirect = () => void

export class BrowserSessionTerminator {
  private active: Promise<void> | undefined
  private redirectRequested = false

  constructor(
    private readonly storage: SessionStorage,
    private readonly clearRuntime: RuntimeClear,
    private readonly clearUserData: UserDataClear,
    private readonly redirectToLogin: LoginRedirect,
  ) {}

  terminate(userUUID: string, redirect: boolean): Promise<void> {
    if (this.active) {
      this.redirectRequested ||= redirect
      return this.active
    }

    const capturedUserUUID = userUUID || this.storedUserUUID()
    this.redirectRequested = redirect
    this.clearCredentials()
    try { this.clearRuntime() } catch { /* persistent cleanup still proceeds */ }

    let cleanup: Promise<void>
    try {
      cleanup = capturedUserUUID ? this.clearUserData(capturedUserUUID) : Promise.resolve()
    } catch {
      cleanup = Promise.resolve()
    }
    this.active = cleanup
      .catch(() => undefined)
      .finally(() => {
        if (this.redirectRequested) this.redirect()
        this.active = undefined
        this.redirectRequested = false
      })
    return this.active
  }

  waitForCleanup(): Promise<void> {
    return this.active ?? Promise.resolve()
  }

  private storedUserUUID() {
    try {
      const user = JSON.parse(this.storage.getItem('dipole.web.user') || '') as { uuid?: unknown }
      return typeof user.uuid === 'string' ? user.uuid : ''
    } catch {
      return ''
    }
  }

  private clearCredentials() {
    for (const key of ['dipole.web.token', 'dipole.web.user', 'dipole.web.lastOfflineID']) {
      try { this.storage.removeItem(key) } catch { /* continue revoking the remaining session state */ }
    }
  }

  private redirect() {
    try { this.redirectToLogin() } catch { /* session remains locally revoked */ }
  }
}
