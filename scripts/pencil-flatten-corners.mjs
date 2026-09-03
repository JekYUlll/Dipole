#!/usr/bin/env node

// Enforces the rectangular BI design rule on design/dipole-ui.pen:
//   • variables.radius-sm / radius-md → 0
//   • variables.radius-pill → 999 (inserted if missing)
//   • every node's cornerRadius is normalized:
//       - if the node is a Pill (name matches /Pill\b/ or radius === 999): keep 999
//       - if the node is Avatar / Dot (name matches /Avatar|Dot/i or radius === "50%"):
//         keep as-is
//       - otherwise: rewrite $radius-sm / $radius-md / any positive numeric radius to 0
//
// Runs a single pass, prints a report, writes atomically. Idempotent.

import { readFileSync, renameSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const target = resolve(process.argv[2] ?? 'design/dipole-ui.pen')
const doc = JSON.parse(readFileSync(target, 'utf8'))

// ---- 1. Variables ------------------------------------------------------------
doc.variables = doc.variables ?? {}
const beforeSm = doc.variables['radius-sm']?.value
const beforeMd = doc.variables['radius-md']?.value
doc.variables['radius-sm'] = { type: 'number', value: 0 }
doc.variables['radius-md'] = { type: 'number', value: 0 }
if (!doc.variables['radius-pill']) {
  doc.variables['radius-pill'] = { type: 'number', value: 999 }
}
console.log(`variables: radius-sm ${beforeSm} → 0, radius-md ${beforeMd} → 0, radius-pill = 999`)

// ---- 2. Node walk ------------------------------------------------------------
const isPillNode = node => {
  if (typeof node.name === 'string' && /Pill\b/i.test(node.name)) return true
  if (node.cornerRadius === 999) return true
  return false
}
const isCircularNode = node => {
  if (typeof node.name === 'string' && /(Avatar|Dot|Circle)/i.test(node.name)) return true
  if (node.cornerRadius === '50%') return true
  return false
}

let changed = 0
let inspected = 0
const changes = []

function walk(node, path) {
  inspected += 1
  if (node && typeof node === 'object' && 'cornerRadius' in node) {
    const current = node.cornerRadius
    if (isPillNode(node) || isCircularNode(node)) {
      // keep round semantic
    } else {
      const shouldFlatten =
        current === '$radius-sm' ||
        current === '$radius-md' ||
        (typeof current === 'number' && current > 0 && current !== 999) ||
        current === 999 // only kept above when it's a Pill/Circular
      if (shouldFlatten) {
        node.cornerRadius = 0
        changed += 1
        changes.push(`${path} (${node.type ?? '?'}${node.name ? ' ' + JSON.stringify(node.name) : ''}) cornerRadius ${JSON.stringify(current)} → 0`)
      }
    }
  }
  if (Array.isArray(node.children)) {
    node.children.forEach((c, i) => walk(c, `${path}/${i}:${c.name ?? c.id ?? c.type}`))
  }
}

doc.children.forEach((c, i) => walk(c, `${i}:${c.name ?? c.id ?? c.type}`))

console.log(`nodes inspected: ${inspected}, cornerRadius rewrites: ${changed}`)
if (changed) {
  for (const line of changes.slice(0, 40)) console.log('  ' + line)
  if (changes.length > 40) console.log(`  … and ${changes.length - 40} more`)
}

// ---- 3. Atomic write ---------------------------------------------------------
const tmp = `${target}.tmp-${process.pid}`
writeFileSync(tmp, JSON.stringify(doc, null, 2), 'utf8')
renameSync(tmp, target)
console.log(`wrote ${target}`)
