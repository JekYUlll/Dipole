import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'SettingsView.vue'), 'utf8')

describe('SettingsView contract', () => {
  it('uses the authenticated profile API and existing device-security route', () => {
    expect(source).toContain('auth.fetchMe()')
    expect(source).toContain('/api/v1/users/${encodeURIComponent(auth.currentUser.uuid)}/profile')
    expect(source).toContain("name: 'devices'")
  })

  it('does not disclose device network or connection identifiers', () => {
    expect(source).not.toMatch(/device\.ip|connection_id|last_seen_at/)
  })

  it('uses the shared V3 design token surface', () => {
    expect(source).toContain('var(--dp-canvas)')
    expect(source).toContain('var(--dp-surface)')
    expect(source).toContain('var(--dp-accent-strong)')
  })
})
