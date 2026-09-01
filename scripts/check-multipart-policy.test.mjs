import test from 'node:test'
import assert from 'node:assert/strict'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const run = promisify(execFile)

test('Multipart policy release manifest is self-consistent', async () => {
  const result = await run(process.execPath, ['scripts/check-multipart-policy.mjs'])
  assert.match(result.stdout, /multipart policy v1 valid sha256=[a-f0-9]{64} mode=relay fallback=relay runtime-defaults=aligned/)
})
