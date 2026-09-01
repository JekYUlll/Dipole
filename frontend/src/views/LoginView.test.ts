import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'LoginView.vue'), 'utf8')

describe('LoginView design contract', () => {
  it('uses the pending V3 brand token surface', () => {
    expect(source).toContain('var(--dp-v3-ivory)')
    expect(source).toContain('var(--dp-v3-navy)')
    expect(source).toContain('var(--dp-v3-red)')
    expect(source).toContain('var(--dp-v3-gold)')
    expect(source).toContain('var(--dp-font-body)')
    expect(source).not.toContain('#07c160')
    expect(source).not.toContain('#e0e0e0')
  })

  it('uses the V3 IM and Agent product marks', () => {
    expect(source).toContain("import dipoleMark from '@/assets/brand/dipole-v3-im.svg'")
    expect(source).toContain("import dipoleAgentMark from '@/assets/brand/dipole-v3-agent.svg'")
    expect(source).toContain('class="brand-mark"')
    expect(source).toContain('IM DATA PLANE / AGENT CONTROL PLANE')
  })
})
