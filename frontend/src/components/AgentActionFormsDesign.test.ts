import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = (name: string) => readFileSync(resolve(import.meta.dirname, name), 'utf8')

describe('Agent action form design contracts', () => {
  it('keeps approval and elicitation surfaces on the shared Pencil token layer', () => {
    for (const name of ['AgentApprovalForm.vue', 'AgentElicitationForm.vue']) {
      const file = source(name)
      expect(file).toContain('var(--dp-canvas)')
      expect(file).toContain('var(--dp-surface)')
      expect(file).toContain('var(--dp-line)')
      expect(file).toContain('var(--dp-font-body)')
      expect(file).toContain('var(--dp-font-display)')
      expect(file).not.toMatch(/--(?:ink|muted|paper|cream|line|accent|forest):\s*#[0-9a-f]{6}/i)
    }
  })
})
