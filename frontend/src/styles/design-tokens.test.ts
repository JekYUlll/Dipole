import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const tokenFile = readFileSync(resolve(import.meta.dirname, 'design-tokens.css'), 'utf8')
const pen = JSON.parse(readFileSync(resolve(import.meta.dirname, '../../../design/dipole-ui.pen'), 'utf8')) as {
  variables: Record<string, { type: string; value: string | number }>
}

function cssToken(name: string): string {
  const match = tokenFile.match(new RegExp(`--dp-${name}:\\s*([^;]+);`))
  if (!match) throw new Error(`Missing Vue token --dp-${name}`)
  return match[1].trim()
}

describe('Pencil design token contract', () => {
  it('keeps the pending V3 brand vocabulary explicit while the canonical Pencil variables migrate', () => {
    expect(cssToken('v3-navy')).toBe('#0b2a4a')
    expect(cssToken('v3-red')).toBe('#f2262a')
    expect(cssToken('v3-gold')).toBe('#f4b000')
    expect(cssToken('v3-ivory')).toBe('#f8f1e4')
  })

  it('keeps canonical color and typography tokens available to Vue', () => {
    const colors = ['canvas', 'surface', 'rail', 'accent', 'danger', 'warning']
    for (const name of colors) expect(cssToken(name)).toBe(String(pen.variables[name].value).toLowerCase())
    expect(cssToken('font-display')).toContain(String(pen.variables['font-display'].value))
    expect(cssToken('font-body')).toContain(String(pen.variables['font-body'].value))
    expect(cssToken('font-data')).toContain(String(pen.variables['font-data'].value))
  })

  it('keeps spacing and radius values aligned with the .pen variables', () => {
    const numericTokens: Record<string, string> = {
      'radius-sm': 'radius-sm', 'radius-md': 'radius-md',
      'space-xs': 'space-xs', 'space-sm': 'space-sm',
      'space-md': 'space-md', 'space-lg': 'space-lg'
    }
    for (const [cssName, penName] of Object.entries(numericTokens)) {
      expect(cssToken(cssName)).toBe(`${pen.variables[penName].value}px`)
    }
  })
})
