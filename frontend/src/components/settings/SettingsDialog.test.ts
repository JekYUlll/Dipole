import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const dialogSrc = readFileSync(resolve(import.meta.dirname, 'SettingsDialog.vue'), 'utf8')
const passwordSrc = readFileSync(resolve(import.meta.dirname, 'ChangePasswordDialog.vue'), 'utf8')

describe('SettingsDialog contract', () => {
  it('uses only the authenticated profile API and the existing device-security route', () => {
    expect(dialogSrc).toContain('auth.fetchMe()')
    expect(dialogSrc).toContain('/api/v1/users/${encodeURIComponent(auth.currentUser.uuid)}/profile')
    expect(dialogSrc).toContain("name: 'devices'")
  })

  it('exposes Agent control links from the shared flag helper', () => {
    expect(dialogSrc).toContain('id="agent-title"')
    expect(dialogSrc).toContain('agentSettingsLinks')
    expect(dialogSrc).toContain('Agent 控制')
  })

  it('does not disclose device network or connection identifiers', () => {
    expect(dialogSrc).not.toMatch(/device\.ip|connection_id|last_seen_at/)
  })

  it('opens a dedicated ChangePasswordDialog rather than inlining password fields', () => {
    expect(dialogSrc).toContain('ChangePasswordDialog')
    expect(dialogSrc).toContain('data-open-change-password')
    // 主设置弹窗不直接采集密码,交给独立 dialog。
    expect(dialogSrc).not.toMatch(/type="password"/)
    expect(dialogSrc).not.toContain('/api/v1/auth/password')
  })

  it('renders as a modal Dialog rather than a full-page settings surface', () => {
    expect(dialogSrc).toContain("import Dialog from 'primevue/dialog'")
    expect(dialogSrc).toContain('data-settings-dialog')
    // 不再有整页 settings-page 布局。
    expect(dialogSrc).not.toContain('.settings-page')
  })
})

describe('ChangePasswordDialog contract', () => {
  it('collects the current password, confirmed replacement, and uses secure autocomplete hints', () => {
    expect(passwordSrc).toContain('autocomplete="current-password"')
    expect(passwordSrc).toContain('autocomplete="new-password"')
    expect(passwordSrc).toContain('v-model="confirmPassword"')
  })

  it('validates locally then calls the protected password endpoint without persisting secrets', () => {
    expect(passwordSrc).toContain('newPassword.value !== confirmPassword.value')
    expect(passwordSrc).toContain("api.patch('/api/v1/auth/password'")
    expect(passwordSrc).toContain('await auth.terminateSession(true)')
    expect(passwordSrc).not.toContain("localStorage.setItem('dipole.web.password'")
  })
})
