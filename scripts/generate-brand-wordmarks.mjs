#!/usr/bin/env node
// Regenerates scripts/brand-wordmarks.json: the Dipole logotype as SVG outlines.
//
// Outlines rather than a font-family reference keep the brand SVGs self-contained,
// so they render identically in GitHub, in the product and in exported images.
//
// The logotype is Goldman Bold: a wide, blocky, hard-edged face whose squared
// counters and heavy uniform stems read as machined hardware, deliberately
// contrasting the mark's discs. Softer geometric faces made the lockup look
// generic. It is markedly wider than a normal grotesque, so the lockup fits its
// logotype by measurement rather than assuming a cap height.
//
// Paths are normalised to cap-height 100 with the baseline on y = 0, so callers
// scale by (target cap height / 100) and never need the source font's metrics.
//
// The type tooling is not a repository dependency; the logotype changes rarely.
// Install it on demand and run this from the repository root:
//
//   npm i --no-save @fontsource/goldman opentype.js wawoff2
//   node scripts/generate-brand-wordmarks.mjs
//   node scripts/generate-brand-assets.mjs
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import opentype from 'opentype.js';
import wawoff2 from 'wawoff2';

const here = dirname(fileURLToPath(import.meta.url));
const FONT = 'goldman/files/goldman-latin-700-normal.woff2';
const LABEL = 'Goldman Bold (@fontsource/goldman 700)';
const CAP_HEIGHT = 100;
// Goldman is already wide and blocky; leave its rhythm alone rather than
// tightening it, which would crowd the squared counters.
const TRACKING = 0;
const TEXTS = { im: 'Dipole IM', agent: 'Dipole Agent', plain: 'Dipole' };

function resolveFont() {
  for (const base of ['node_modules/@fontsource', join(here, '..', 'node_modules', '@fontsource')]) {
    try {
      return readFileSync(join(base, FONT));
    } catch {
      /* try the next location */
    }
  }
  throw new Error(`Cannot find @fontsource/${FONT}. Run: npm i --no-save @fontsource/goldman opentype.js wawoff2`);
}

const font = opentype.parse(new Uint8Array(await wawoff2.decompress(resolveFont())).buffer);
const capHeight = font.tables.os2.sCapHeight;
if (!capHeight) throw new Error('font exposes no cap height; cannot normalise');
const size = (CAP_HEIGHT * font.unitsPerEm) / capHeight;
const scale = size / font.unitsPerEm;
const n = (v) => Number(v.toFixed(2)).toString();

function outline(text) {
  const glyphs = [...text].map((ch) => font.charToGlyph(ch));
  const parts = [];
  let x = 0;
  for (const [i, glyph] of glyphs.entries()) {
    if (i > 0) x += font.getKerningValue(glyphs[i - 1], glyph) * scale + TRACKING;
    for (const c of glyph.getPath(x, 0, size).commands) {
      if (c.type === 'M') parts.push(`M${n(c.x)} ${n(c.y)}`);
      else if (c.type === 'L') parts.push(`L${n(c.x)} ${n(c.y)}`);
      else if (c.type === 'C') parts.push(`C${n(c.x1)} ${n(c.y1)} ${n(c.x2)} ${n(c.y2)} ${n(c.x)} ${n(c.y)}`);
      else if (c.type === 'Q') parts.push(`Q${n(c.x1)} ${n(c.y1)} ${n(c.x)} ${n(c.y)}`);
      else if (c.type === 'Z') parts.push('Z');
      else throw new Error(`unexpected path command ${c.type}`);
    }
    x += glyph.advanceWidth * scale;
  }
  const d = parts.join('');
  if (d.includes('NaN')) throw new Error(`outline for "${text}" is not finite`);
  return { width: Number(x.toFixed(2)), d };
}

const payload = {
  font: LABEL,
  capHeight: CAP_HEIGHT,
  tracking: TRACKING,
  marks: Object.fromEntries(Object.entries(TEXTS).map(([key, text]) => [key, outline(text)])),
};
const target = join(here, 'brand-wordmarks.json');
writeFileSync(target, `${JSON.stringify(payload, null, 2)}\n`);
console.log(`wrote scripts/brand-wordmarks.json from ${LABEL}`);
for (const [key, mark] of Object.entries(payload.marks)) {
  console.log(`  ${key}: width ${mark.width} at cap-height ${CAP_HEIGHT}`);
}
