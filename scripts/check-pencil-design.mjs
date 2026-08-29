#!/usr/bin/env node

import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, resolve } from 'node:path'

const file = resolve(process.argv[2] ?? 'design/dipole-ui.pen')
const manifestFile = resolve(process.env.DIPOLE_DESIGN_EXPORT_MANIFEST ?? resolve(dirname(file), 'export-manifest.json'))
let document
try {
  document = JSON.parse(readFileSync(file, 'utf8'))
} catch (error) {
  fail(`cannot read valid JSON: ${error instanceof Error ? error.message : String(error)}`)
}

if (!document || typeof document !== 'object' || document.version === undefined || !Array.isArray(document.children)) {
  fail('canonical Pencil document must contain version and children')
}
if (!document.variables || typeof document.variables !== 'object' || Object.keys(document.variables).length === 0) {
  fail('canonical Pencil document must define design variables')
}

checkExportManifest()

const nodes = []
walk(document)
const names = new Set(nodes.map(node => node.name).filter(name => typeof name === 'string'))
const requiredFrames = [
  '00 Foundations', 'Login Desktop', 'Login Mobile', 'Chat Desktop', 'Chat Mobile',
  'Search/Desktop/Results', 'Search/Mobile/Results', 'Sync/Desktop/Restoring', 'Sync/Mobile/Restoring',
  'Agent Repair/Desktop/Proposed', 'Agent Repair/Mobile/Approval',
  'Agent Elicitation/Desktop/Form', 'Agent Elicitation/Mobile/Form',
  'Agent Subscription/Desktop/Manage', 'Agent Subscription/Mobile/Revoke',
  'Agent Memory/Desktop/Manage', 'Agent Memory/Mobile/Revoke'
]
const missing = requiredFrames.filter(name => !names.has(name))
if (missing.length > 0) fail(`required design frames are missing: ${missing.join(', ')}`)

const placeholders = nodes.filter(node => node.placeholder === true)
if (placeholders.length > 0) fail(`design contains ${placeholders.length} placeholder node(s)`)
const unnamed = nodes.filter(node => typeof node.name === 'string' && /^(Unnamed|Frame \d+)$/.test(node.name))
if (unnamed.length > 0) fail(`design contains ${unnamed.length} unnamed node(s)`)

const reusable = nodes.filter(node => node.reusable === true).length
console.log(`Pencil design gate passed: frames=${document.children.length} nodes=${nodes.length} variables=${Object.keys(document.variables).length} reusable=${reusable}`)

function walk(value) {
  if (!value || typeof value !== 'object') return
  if (Array.isArray(value)) {
    value.forEach(walk)
    return
  }
  if ('id' in value || 'name' in value || 'type' in value) nodes.push(value)
  Object.values(value).forEach(walk)
}

function fail(message) {
  process.stderr.write(`Pencil design gate: ${message}\n`)
  process.exit(1)
}

function checkExportManifest() {
  let manifest
  try {
    manifest = JSON.parse(readFileSync(manifestFile, 'utf8'))
  } catch (error) {
    fail(`cannot read export manifest: ${error instanceof Error ? error.message : String(error)}`)
  }
  if (!manifest || manifest.version !== 1 || !Array.isArray(manifest.exports) || manifest.exports.length === 0) {
    fail('export manifest must contain version 1 and at least one export')
  }

  for (const entry of manifest.exports) {
    if (!entry || typeof entry.path !== 'string' || entry.path.length === 0) {
      fail('export manifest contains an invalid entry')
    }
    const designDir = dirname(manifestFile)
    const target = resolve(designDir, entry.path)
    if (!target.startsWith(designDir + '/')) {
      fail(`export manifest path escapes design directory: ${entry.path}`)
    }
    let stats
    try {
      stats = statSync(target)
    } catch {
      fail(`approved export is missing: ${entry.path}`)
    }
    if (stats.isDirectory()) {
      const files = readdirSync(target, { withFileTypes: true })
        .filter(item => item.isFile() && item.name.toLowerCase().endsWith('.png'))
      if (files.length === 0) fail(`approved export directory has no PNG files: ${entry.path}`)
      continue
    }
    if (!stats.isFile() || stats.size === 0) fail(`approved export is empty: ${entry.path}`)
  }
}
