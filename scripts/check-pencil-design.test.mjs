import assert from 'node:assert/strict'
import { chmodSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { execFileSync } from 'node:child_process'
import test from 'node:test'

const script = new URL('./check-pencil-design.mjs', import.meta.url)

test('accepts the canonical Pencil design baseline', () => {
  const output = execFileSync(process.execPath, [script.pathname, new URL('../design/dipole-ui.pen', import.meta.url).pathname], { encoding: 'utf8' })
  assert.match(output, /Pencil design gate passed/)
})

test('rejects missing required frames', () => {
  const root = mkdtempSync(join(tmpdir(), 'dipole-pencil-gate-'))
  const file = join(root, 'invalid.pen')
  writeFileSync(file, JSON.stringify({ version: 2, variables: { ink: '#000' }, children: [{ id: 'x', name: 'Login Desktop', placeholder: true }] }))
  assert.throws(
    () => execFileSync(process.execPath, [script.pathname, file], { stdio: 'pipe' }),
    error => error.status === 1,
  )
})
