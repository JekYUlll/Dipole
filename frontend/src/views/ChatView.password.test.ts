import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'ChatView.vue'), 'utf8')

describe('ChatView password change contract', () => {
  it('collects the current password, confirmed replacement, and uses secure autocomplete hints', () => {
    expect(source).toContain('autocomplete="current-password"')
    expect(source).toContain('autocomplete="new-password"')
    expect(source).toContain('v-model="confirmPassword"')
  })

  it('validates locally then calls the protected password endpoint without persisting secrets', () => {
    expect(source).toContain("newPassword.value !== confirmPassword.value")
    expect(source).toContain("api.patch('/api/v1/auth/password'")
    expect(source).toContain('await auth.terminateSession(true)')
    expect(source).not.toContain('localStorage.setItem(\'dipole.web.password\'')
  })
})
