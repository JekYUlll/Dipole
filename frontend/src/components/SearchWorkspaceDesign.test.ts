import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(import.meta.dirname, 'SearchWorkspace.vue'), 'utf8')

describe('SearchWorkspace design contracts', () => {
  it('uses the shared Pencil token layer for visual surfaces', () => {
    expect(source).toContain('--search-canvas: var(--dp-canvas)')
    expect(source).toContain('--search-surface: var(--dp-surface)')
    expect(source).toContain('--search-accent-soft: var(--dp-accent-soft)')
    expect(source).toContain('font-family: var(--dp-font-body)')
    expect(source).not.toMatch(/(?:background|color|border-color):\s*#[0-9a-f]{6}/i)
  })
})
