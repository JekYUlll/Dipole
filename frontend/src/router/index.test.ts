import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'index.ts'), 'utf8')

describe('Route security contract', () => {
  it('keeps Chat as the only Agent surface and drops standalone /agent/* pages', () => {
    expect(source).toContain("name: 'chat'")
    expect(source).toContain("path: '/'")
    expect(source).not.toContain("path: '/agent/")
    expect(source).not.toContain("name: 'agent-")
  })

  it('redirects legacy /settings to Chat with the settings=1 modal trigger', () => {
    expect(source).toContain("path: '/settings'")
    expect(source).toContain("redirect: { path: '/', query: { settings: '1' } }")
    // Settings 不再作为独立页面组件被引入。
    expect(source).not.toContain('SettingsView.vue')
  })

  it('keeps the Contact directory authenticated without an Agent feature flag', () => {
    expect(source).toContain("name: 'contacts'")
    expect(source).toContain("path: '/contacts'")
  })

  it('keeps the Group directory authenticated without a write feature flag', () => {
    expect(source).toContain("name: 'groups'")
    expect(source).toContain("path: '/groups'")
  })

  it('keeps the owner file directory authenticated without exposing upload controls', () => {
    expect(source).toContain("name: 'files'")
    expect(source).toContain("path: '/files'")
  })

  it('keeps Device Security authenticated and preserves the explicit logout-other-device semantics', () => {
    expect(source).toContain("name: 'devices'")
    expect(source).toContain("path: '/devices'")
  })

  it('keeps unauthenticated access redirected to Login', () => {
    expect(source).toContain('if (to.meta.requiresAuth && !auth.token)')
    expect(source).toContain("return { name: 'login' }")
  })
})
