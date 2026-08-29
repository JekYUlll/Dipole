#!/usr/bin/env node

import { existsSync, mkdirSync, readFileSync, renameSync, rmSync, statSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { spawn } from 'node:child_process'

const args = process.argv.slice(2)
const input = option(args, '--input')
const output = option(args, '--output')
const exportPath = option(args, '--export')
const timeoutMs = Number.parseInt(option(args, '--timeout-ms') ?? '120000', 10)
const separator = args.indexOf('--')
const penArgs = separator < 0 ? [] : args.slice(separator + 1)
const prompt = option(penArgs, '--prompt') ?? option(penArgs, '-p')

if (!input || !output || !Number.isSafeInteger(timeoutMs) || timeoutMs < 1_000 || penArgs.length === 0 || !prompt?.trim()) {
  fail('usage: pencil-safe-edit.mjs --input <file> --output <file> [--export <image>] [--timeout-ms <ms>] -- --prompt <text> [pen arguments]')
}

const inputPath = resolve(input)
const outputPath = resolve(output)
if (!existsSync(inputPath)) fail('Pencil input does not exist')

const temporaryPath = `${outputPath}.tmp-${process.pid}`
mkdirSync(dirname(outputPath), { recursive: true })
rmSync(temporaryPath, { force: true })

const childArgs = ['--in', inputPath, '--out', temporaryPath, ...penArgs]
if (exportPath) childArgs.push('--export', resolve(exportPath))

const child = spawn('pen', childArgs, { stdio: 'inherit' })
let settled = false
const timer = setTimeout(() => {
  if (settled) return
  child.kill('SIGTERM')
  setTimeout(() => child.kill('SIGKILL'), 2_000).unref()
  finish(new Error(`Pencil edit timed out after ${timeoutMs}ms`), 124)
}, timeoutMs)

child.on('error', error => finish(error, 1))
child.on('exit', (code, signal) => {
  if (settled) return
  if (code !== 0) finish(new Error(`Pencil exited with ${code ?? 'signal ' + signal}`), 1)
  else validateAndCommit()
})

function validateAndCommit() {
  try {
    const document = JSON.parse(readFileSync(temporaryPath, 'utf8'))
    if (!document || typeof document !== 'object' || document.version === undefined || !Array.isArray(document.children)) {
      throw new Error('Pencil output is not a valid .pen document')
    }
    if (exportPath && (!existsSync(resolve(exportPath)) || statSync(resolve(exportPath)).size === 0)) {
      throw new Error('Pencil export was not produced')
    }
    renameSync(temporaryPath, outputPath)
    finish()
  } catch (error) {
    finish(error, 1)
  }
}

function finish(error, code = error ? 1 : 0) {
  if (settled) return
  settled = true
  clearTimeout(timer)
  rmSync(temporaryPath, { force: true })
  if (error) {
    process.stderr.write(`pencil-safe-edit: ${error.message}\n`)
    process.exitCode = code
  }
}

function option(values, name) {
  const index = values.indexOf(name)
  return index < 0 ? undefined : values[index + 1]
}

function fail(message) {
  process.stderr.write(`pencil-safe-edit: ${message}\n`)
  process.exit(2)
}
