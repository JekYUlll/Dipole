import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'ChatView.vue'), 'utf8')

describe('ChatView V3 design contract', () => {
  it('uses the V3 mark and layered product-workspace palette', () => {
    expect(source).toContain("const dipoleMark = `${import.meta.env.BASE_URL}dipole-v3-im.svg`")
    expect(source).toContain('class="nav-brand-mark"')
    expect(source).toContain('var(--dp-v3-navy)')
    expect(source).toContain('var(--dp-v3-ivory)')
    expect(source).toContain('var(--dp-v3-red)')
    expect(source).toContain('var(--dp-v3-gold-soft)')
  })

  it('keeps synchronization and user-authorized agent entry points visible', () => {
    expect(source).toContain('data-agent-task-create-entry')
    expect(source).toContain('安全同步游标')
    expect(source).toContain('class="sync-status"')
  })
})
