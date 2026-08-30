import assert from 'node:assert/strict'
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { execFileSync } from 'node:child_process'
import test from 'node:test'

const wrapper = new URL('./pencil-safe-edit.mjs', import.meta.url)

test('commits a valid Pencil output and export atomically', () => {
  const root = fixture()
  const output = join(root, 'output.pen')
  const exportPath = join(root, 'preview.png')

  runFakePen(root, 'success', output, exportPath)

  assert.deepEqual(JSON.parse(readFileSync(output, 'utf8')).children, [])
  assert.equal(readFileSync(exportPath, 'utf8'), 'png')
  assertNoTempFiles(root)
})

test('keeps the canonical file when Pencil times out', () => {
  const root = fixture()
  const output = join(root, 'output.pen')
  writeFileSync(output, '{"version":1,"children":[{"name":"canonical"}]}')

  assert.throws(
    () => runFakePen(root, 'hang', output),
    error => error.status === 124,
  )

  assert.match(readFileSync(output, 'utf8'), /canonical/)
  assertNoTempFiles(root)
})

test('uses a brief file as the Pencil prompt', () => {
  const root = fixture()
  const output = join(root, 'output.pen')
  const brief = join(root, 'brief.md')
  writeFileSync(brief, 'brief prompt')

  runFakePen(root, 'success', output, undefined, ['--prompt-file', brief], false)

  assert.deepEqual(JSON.parse(readFileSync(output, 'utf8')).children, [])
  assertNoTempFiles(root)
})

test('rejects a Pencil invocation without a prompt before spawning the CLI', () => {
  const root = fixture()
  const output = join(root, 'output.pen')

  assert.throws(
    () => runFakePen(root, 'success', output, undefined, [], false),
    error => error.status === 2 && /--prompt/.test(error.stderr.toString()),
  )

  assert.equal(existsSync(output), false)
  assertNoTempFiles(root)
})

function fixture() {
  const root = mkdtempSync(join(tmpdir(), 'dipole-pencil-safe-edit-'))
  const input = join(root, 'input.pen')
  writeFileSync(input, '{"version":1,"children":[]}')
  return root
}

function runFakePen(root, mode, output, exportPath, extraArgs = [], includePrompt = true) {
  const bin = join(root, 'bin')
  const pen = join(bin, 'pen')
  const input = join(root, 'input.pen')
  const args = [wrapper.pathname, '--input', input, '--output', output, '--timeout-ms', '1000']
  if (exportPath) args.push('--export', exportPath)
  args.push('--')
  if (includePrompt) args.push('--prompt', 'test prompt')
  args.push('--mode', mode, ...extraArgs)

  mkdirSync(bin)
  writeFileSync(pen, `#!/usr/bin/env node
const fs = require('node:fs')
const args = process.argv.slice(2)
if (args.includes('--mode') && args[args.indexOf('--mode') + 1] === 'hang') setTimeout(() => {}, 10000)
const out = args[args.indexOf('--out') + 1]
if (!args.includes('--mode') || args[args.indexOf('--mode') + 1] !== 'hang') {
  fs.writeFileSync(out, JSON.stringify({ version: 1, children: [] }))
  const exportIndex = args.indexOf('--export')
  if (exportIndex >= 0) fs.writeFileSync(args[exportIndex + 1], 'png')
}
`)
  chmodSync(pen, 0o755)
  return execFileSync(process.execPath, args, {
    env: { ...process.env, PATH: `${bin}:${process.env.PATH}` },
    stdio: 'pipe',
  })
}

function assertNoTempFiles(root) {
  assert.equal(readdirSync(root).some(name => name.includes('.tmp-')), false)
  assert.equal(existsSync(join(root, 'output.pen.tmp')), false)
}
