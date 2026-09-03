#!/usr/bin/env node

// Purges design/dipole-ui.pen down to the 5 frames that reflect the ratified
// "Chat + Agent Drawer" IA (docs/notes/frontend-bi-redesign.md §3.1):
//
//   0. App Chrome v2
//   1. Chat + Agent Drawer · Live
//   2. Chat + Agent Drawer · Tasks
//   3. State Patterns v1
//   4. BI Foundations v1                    (component reference, kept)
//
// Every other top-level frame (108 legacy frames including 15 "State Matrix"
// solo pages and every mobile view) is deleted. This is deliberate: the old
// frames encode the "Agent as independent multi-page" IA that we killed.
//
// A backup is written alongside the target. Atomic rename on write.

import { readFileSync, writeFileSync, renameSync, copyFileSync } from 'node:fs'
import { resolve } from 'node:path'

const KEEP = [
  'App Chrome v2',
  'Chat + Agent Drawer · Live',
  'Chat + Agent Drawer · Tasks',
  'State Patterns v1',
  'BI Foundations v1',
]

const target = resolve(process.argv[2] ?? 'design/dipole-ui.pen')
const doc = JSON.parse(readFileSync(target, 'utf8'))
if (!doc || !Array.isArray(doc.children)) {
  console.error('pen file has no children[]')
  process.exit(1)
}

const beforeCount = doc.children.length
const beforeSize = readFileSync(target, 'utf8').length

// Backup adjacent (workspace-safe path)
const stamp = new Date().toISOString().replace(/[:.]/g, '-')
const backup = `${target}.backup-${stamp}`
copyFileSync(target, backup)

// Bucket by "keep or drop" so we can report what left
const kept = []
const dropped = []
for (const c of doc.children) {
  const name = (c && c.name) || '?'
  if (KEEP.includes(name)) {
    kept.push(c)
  } else {
    dropped.push(name)
  }
}

// Sort kept in canonical order matching KEEP so first frame in the viewer
// is always "App Chrome v2".
kept.sort((a, b) => KEEP.indexOf(a.name) - KEEP.indexOf(b.name))

doc.children = kept

const tmp = `${target}.tmp-${process.pid}`
writeFileSync(tmp, JSON.stringify(doc, null, 2), 'utf8')
renameSync(tmp, target)

const afterSize = readFileSync(target, 'utf8').length
console.log(`Backup: ${backup}`)
console.log(`Before: ${beforeCount} frames, ${(beforeSize / 1024).toFixed(1)} KB`)
console.log(`After : ${kept.length} frames, ${(afterSize / 1024).toFixed(1)} KB`)
console.log()
console.log('Kept (new order):')
for (let i = 0; i < kept.length; i++) console.log(`  [${i}] ${kept[i].name}`)
console.log()
console.log(`Dropped ${dropped.length} legacy frames, incl.`)
for (const name of dropped.filter(n => /State Matrix/i.test(n))) {
  console.log(`  · ${name}   (你点名的类型)`)
}
