import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'LoginView.vue'), 'utf8')

describe('LoginView design contract', () => {
  it('uses the shared Pencil token surface', () => {
    expect(source).toContain('var(--dp-canvas)')
    expect(source).toContain('var(--dp-surface)')
    expect(source).toContain('var(--dp-accent)')
    expect(source).toContain('var(--dp-font-body)')
    expect(source).not.toContain('#07c160')
    expect(source).not.toContain('#e0e0e0')
  })

  it('uses the canonical Signal Link product mark', () => {
    expect(source).toContain("import dipoleMark from '../../../docs/images/dipole-mark.svg'")
    expect(source).toContain('class="brand-mark"')
    expect(source).toContain('Realtime collaboration')
  })
})
