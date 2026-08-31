import assert from 'node:assert/strict'
import { cpSync, mkdtempSync, rmSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'

const script = new URL('./check-brand-assets.mjs', import.meta.url)
const kit = new URL('../design/brand/', import.meta.url)

test('accepts the canonical Dipole V3 brand kit', () => {
  const output = execFileSync(process.execPath, [script.pathname, kit.pathname], { encoding: 'utf8' })
  assert.match(output, /Brand asset gate passed: source=3 variants=5 references=1/)
})

test('rejects a missing required brand asset', () => {
  const root = mkdtempSync(join(tmpdir(), 'dipole-brand-gate-'))
  const candidate = join(root, 'brand')
  cpSync(kit.pathname, candidate, { recursive: true })
  rmSync(join(candidate, 'variants', 'dipole-v3-favicon.svg'))
  assert.throws(
    () => execFileSync(process.execPath, [script.pathname, candidate], { stdio: 'pipe' }),
    error => error.status === 1,
  )
})
