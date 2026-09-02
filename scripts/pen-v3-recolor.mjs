#!/usr/bin/env node

// Deterministic V3 recolor for the canonical Pencil design source.
//
// design/dipole-ui.pen carried two generations of palette: the current V3
// variables (rail/ink/accent/agent/canvas/line ...) plus a leftover set of
// legacy variables and ~240 inline fill/stroke hexes from the retired
// teal/sage/telegram palette. This script closes that debt without any AI /
// non-deterministic edit so the change reviews as a clean value diff:
//
//   1. repoint frame references from the 6 legacy variables to their V3 twins
//      and drop the now-unused legacy variables,
//   2. add success/success-soft to mirror the app token --dp-success,
//   3. remap every inline fill/stroke hex to the V3 variable that carries its
//      role (pure #FFFFFF / #000000 stay inline as palette-neutral),
//   4. assert nothing is left unmapped and no dangling legacy reference remains.
//
// Colours only ever live on `fill` and `stroke`; `content` (e.g. the literal
// "#288" order labels) and `fontFamily` are never touched. Rendering the
// approved exports is a separate step (pen Export); this script only rewrites
// the JSON, byte-for-byte compatible with `JSON.stringify(doc, null, 2)`.

import { readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const args = process.argv.slice(2)
const CHECK = args.includes('--check')
const FILE = resolve(args.find(a => !a.startsWith('--')) ?? 'design/dipole-ui.pen')

const COLOR_KEYS = new Set(['fill', 'stroke'])

// Legacy design variables -> canonical V3 semantic variables.
const VAR_REPOINT = {
  'text-primary': 'ink',
  'text-secondary': 'ink-soft',
  'text-muted': 'ink-faint',
  'border': 'line',
  'surface-app': 'canvas',
  'surface-inverse': 'rail',
}

// New functional variables, aligned to the app's --dp-success token.
const ADD_VARS = {
  'success': { type: 'color', value: '#1F8A54' },
  'success-soft': { type: 'color', value: '#E6F4EC' },
}

// Inline fill/stroke hexes -> V3 variable, grouped by the role each colour
// played in the retired palette. Structure reads navy, identity/progress reads
// gold, positive state reads the muted BI success green, attention reads amber,
// destructive reads danger; the many sage/telegram neutrals fold onto the warm
// ivory line and ink scales.
const HEX_MAP = {
  // navy structure / dark surfaces
  '#172126': 'rail', '#102F40': 'rail', '#111827': 'rail', '#0C354C': 'rail',
  '#223044': 'rail', '#314039': 'rail', '#1A675D': 'rail', '#315B58': 'rail-soft',
  // ink text
  '#5F7773': 'ink-soft', '#51615B': 'ink-soft',
  '#91A09B': 'ink-faint', '#AAB7B1': 'ink-faint', '#B8C5C0': 'ink-faint',
  '#B8C3BE': 'ink-faint', '#CBD5E1': 'ink-faint', '#B6C2CF': 'ink-faint',
  '#93B8B0': 'ink-faint', '#9AB1A6': 'ink-faint',
  // warm ivory lines / dividers
  '#D9DEDA': 'line', '#DDE2DC': 'line', '#B7C7C0': 'line', '#D5DEE8': 'line',
  '#C9D6D0': 'line', '#A5C5BC': 'line', '#D7DEDA': 'line', '#D6DED9': 'line',
  '#C6D2CE': 'line', '#C3D1CA': 'line',
  // muted / paper surfaces
  '#E7E9E5': 'surface-muted', '#EEF3F1': 'surface-muted', '#E7EAE6': 'surface-muted',
  '#EEF0ED': 'surface-muted', '#E0F2FE': 'surface-muted',
  '#F7F4EC': 'canvas', '#F4F0E7': 'canvas',
  '#F9FAF8': 'surface', '#FBFCFA': 'surface', '#FFFCF6': 'surface', '#FFFDF7': 'surface',
  // agent gold
  '#F0C879': 'agent', '#FEC84B': 'agent',
  '#FFF7EA': 'agent-soft', '#FFF6E8': 'agent-soft', '#F2D7A7': 'agent-soft',
  // warning amber
  '#805312': 'warning', '#E47A36': 'warning', '#D66E2E': 'warning',
  '#FFFAEB': 'warning-soft', '#FEF3C7': 'warning-soft',
  // success green
  '#1F3A2D': 'success', '#83E3B7': 'success',
  '#D1FADF': 'success-soft', '#EAF5F0': 'success-soft', '#EAF6EF': 'success-soft',
  '#DDF5E9': 'success-soft', '#DDEFEA': 'success-soft', '#B8E3CB': 'success-soft',
  // danger red
  '#FDA29B': 'danger', '#FF9C94': 'danger', '#FEF3F2': 'danger-soft',
  '#FCEDEA': 'accent-soft', '#FCE7F3': 'accent-soft',
}

// Palette-neutral inline colours that are allowed to stay hard-coded.
const ALLOW_INLINE = new Set(['#FFFFFF', '#000000'])

const raw = readFileSync(FILE, 'utf8')
const doc = JSON.parse(raw)

const counters = { varRepoint: {}, hexRemap: {} }
const unmapped = new Map()

walk(doc.children)

const unmappedList = [...unmapped.entries()].sort((a, b) => b[1] - a[1])
if (unmappedList.length > 0) {
  fail(`unmapped inline fill/stroke hex(es): ${unmappedList.map(([h, n]) => `${h}x${n}`).join(', ')}`)
}

// Variables: drop the legacy set, add the success pair.
const removedVars = []
for (const legacy of Object.keys(VAR_REPOINT)) {
  if (legacy in doc.variables) { delete doc.variables[legacy]; removedVars.push(legacy) }
}
const addedVars = []
for (const [name, def] of Object.entries(ADD_VARS)) {
  if (!(name in doc.variables)) { doc.variables[name] = def; addedVars.push(name) }
}

const out = JSON.stringify(doc, null, 2)

// No dangling reference to any removed variable may survive anywhere.
for (const legacy of removedVars) {
  if (out.includes(`"$${legacy}"`)) fail(`dangling reference to removed variable $${legacy}`)
}
// No mapped legacy hex may survive on a fill/stroke.
const survivors = new Set()
walkSurvivors(JSON.parse(out))
if (survivors.size > 0) fail(`fill/stroke still holds mapped hex(es): ${[...survivors].join(', ')}`)

const changed = out !== raw
if (CHECK) {
  if (changed) fail('design source is not V3-normalised (run without --check to fix)')
  console.log('pen-v3-recolor: design source already V3-normalised')
  process.exit(0)
}

writeFileSync(FILE, out)
report()

function walk(value) {
  if (!value || typeof value !== 'object') return
  if (Array.isArray(value)) { value.forEach(walk); return }
  for (const [key, val] of Object.entries(value)) {
    if (COLOR_KEYS.has(key) && typeof val === 'string') {
      value[key] = recolor(val)
    } else if (!COLOR_KEYS.has(key)) {
      walk(val)
    }
  }
}

function recolor(val) {
  const refMatch = val.match(/^\$([a-z0-9-]+)$/i)
  if (refMatch) {
    const target = VAR_REPOINT[refMatch[1]]
    if (target) { counters.varRepoint[refMatch[1]] = (counters.varRepoint[refMatch[1]] || 0) + 1; return `$${target}` }
    return val
  }
  const hexMatch = val.match(/^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/)
  if (hexMatch) {
    const hex = val.toUpperCase()
    if (ALLOW_INLINE.has(hex)) return val
    const target = HEX_MAP[hex]
    if (target) { counters.hexRemap[target] = (counters.hexRemap[target] || 0) + 1; return `$${target}` }
    unmapped.set(hex, (unmapped.get(hex) || 0) + 1)
    return val
  }
  return val
}

function walkSurvivors(value) {
  if (!value || typeof value !== 'object') return
  if (Array.isArray(value)) { value.forEach(walkSurvivors); return }
  for (const [key, val] of Object.entries(value)) {
    if (COLOR_KEYS.has(key) && typeof val === 'string') {
      const hex = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(val) ? val.toUpperCase() : null
      if (hex && HEX_MAP[hex]) survivors.add(hex)
    } else if (!COLOR_KEYS.has(key)) {
      walkSurvivors(val)
    }
  }
}

function report() {
  const varTotal = Object.values(counters.varRepoint).reduce((s, n) => s + n, 0)
  const hexTotal = Object.values(counters.hexRemap).reduce((s, n) => s + n, 0)
  console.log(`pen-v3-recolor: repointed ${varTotal} legacy variable reference(s)`) 
  for (const [k, n] of Object.entries(counters.varRepoint).sort((a, b) => b[1] - a[1])) {
    console.log(`  $${k} -> $${VAR_REPOINT[k]}  x${n}`)
  }
  console.log(`pen-v3-recolor: remapped ${hexTotal} inline fill/stroke hex(es) across ${Object.keys(HEX_MAP).length} colour(s)`) 
  for (const [k, n] of Object.entries(counters.hexRemap).sort((a, b) => b[1] - a[1])) {
    console.log(`  -> $${k}  x${n}`)
  }
  console.log(`pen-v3-recolor: removed variables [${removedVars.join(', ')}], added [${addedVars.join(', ')}]`)
}

function fail(message) {
  process.stderr.write(`pen-v3-recolor: ${message}\n`)
  process.exit(1)
}
