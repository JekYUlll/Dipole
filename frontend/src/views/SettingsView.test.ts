import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'SettingsView.vue'), 'utf8')

describe('SettingsView contract', () => {
  it('uses authenticated profile and device-session APIs only', () => {
    expect(source).toContain("auth.fetchMe()")
    expect(source).toContain("chat.fetchDevices()")
    expect(source).toContain("/api/v1/users/${encodeURIComponent(auth.currentUser.uuid)}/profile")
    expect(source).toContain('/api/v1/users/me/devices/${encodeURIComponent(connectionID)}/logout')
    expect(source).toContain('/api/v1/users/me/devices/logout-all')
  })

  it('keeps destructive device actions behind explicit confirmation', () => {
    expect(source).toContain("window.confirm('确认移除这个设备吗？该设备会立即下线。')")
    expect(source).toContain("window.confirm('确认退出所有设备吗？本设备也会退出。')")
  })

  it('uses the shared design token surface', () => {
    expect(source).toContain('var(--dp-canvas)')
    expect(source).toContain('var(--dp-surface)')
    expect(source).toContain('var(--dp-accent)')
  })
})
