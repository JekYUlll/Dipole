#!/usr/bin/env node

import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const file = resolve(process.argv[2] ?? 'design/dipole-ui.pen')
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
