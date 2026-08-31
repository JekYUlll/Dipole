#!/usr/bin/env node

import { readFileSync, statSync } from 'node:fs'
import { resolve } from 'node:path'

const kitDir = resolve(process.argv[2] ?? 'design/brand')
const manifestPath = resolve(kitDir, 'manifest.json')
const projectRoot = resolve(kitDir, '../..')
const manifest = readJson(manifestPath, 'brand manifest')

if (manifest.version !== 1 || !Array.isArray(manifest.sourceAssets) || !Array.isArray(manifest.variants) || !Array.isArray(manifest.referenceAssets)) {
  fail('manifest must define version 1 plus sourceAssets, variants, and referenceAssets arrays')
}

const assets = [...manifest.sourceAssets, ...manifest.variants, ...manifest.referenceAssets]
if (assets.length === 0 || assets.some(asset => typeof asset !== 'string' || asset.length === 0)) {
  fail('manifest contains an invalid asset path')
}

for (const asset of assets) {
  const file = insideKit(asset)
  if (!isNonEmptyFile(file)) fail(`required asset is missing or empty: ${asset}`)
  if (asset.endsWith('.svg')) validateSvg(asset, readFileSync(file, 'utf8'))
}

if (!manifest.publishedCopies || typeof manifest.publishedCopies !== 'object') {
  fail('manifest must define publishedCopies')
}
for (const [source, published] of Object.entries(manifest.publishedCopies)) {
  if (!manifest.sourceAssets.includes(source) || typeof published !== 'string') {
    fail(`invalid published copy mapping: ${source}`)
  }
  const sourceFile = insideKit(source)
  const publishedFile = resolve(projectRoot, published)
  if (!publishedFile.startsWith(projectRoot + '/')) fail(`published path escapes project root: ${published}`)
  if (!isNonEmptyFile(publishedFile)) fail(`published copy is missing or empty: ${published}`)
  if (readFileSync(sourceFile, 'utf8') !== readFileSync(publishedFile, 'utf8')) {
    fail(`published copy drifted from canonical source: ${published}`)
  }
}

const colors = manifest.colors
if (!colors || colors.navy !== '#0B2A4A' || colors.signalRed !== '#F2262A' || colors.orbitGold !== '#F4B000' || colors.ivory !== '#F8F1E4') {
  fail('manifest must retain the approved V3 navy, red, gold, and ivory values')
}

console.log(`Brand asset gate passed: source=${manifest.sourceAssets.length} variants=${manifest.variants.length} references=${manifest.referenceAssets.length}`)

function insideKit(relativePath) {
  const file = resolve(kitDir, relativePath)
  if (!file.startsWith(kitDir + '/')) fail(`asset path escapes brand kit: ${relativePath}`)
  return file
}

function validateSvg(relativePath, content) {
  if (!/<svg\b[^>]*\bviewBox="[^"]+"/.test(content)) fail(`SVG must define a viewBox: ${relativePath}`)
  if (/<image\b/i.test(content)) fail(`SVG must remain vector-only: ${relativePath}`)
  if (!/<title\b/.test(content) || !/<desc\b/.test(content)) fail(`SVG must provide title and description: ${relativePath}`)
}

function isNonEmptyFile(file) {
  try {
    return statSync(file).isFile() && statSync(file).size > 0
  } catch {
    return false
  }
}

function readJson(file, label) {
  try {
    return JSON.parse(readFileSync(file, 'utf8'))
  } catch (error) {
    fail(`cannot read ${label}: ${error instanceof Error ? error.message : String(error)}`)
  }
}

function fail(message) {
  process.stderr.write(`Brand asset gate: ${message}\n`)
  process.exit(1)
}
