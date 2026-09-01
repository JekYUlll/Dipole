#!/usr/bin/env node
// Generates the Dipole V3 brand assets in docs/images/ from a single geometry
// definition, so every mark, lockup and favicon stays dimensionally identical.
//
// Geometry was measured from docs/images/dipole-brand-v3.png (the authored brand
// board) and then regularised: the board is a raster with a ~4px asymmetry
// between the two bubbles, which is normalised here into an exactly mirrored
// construction. Wordmark outlines are Poppins Bold converted to paths so the
// assets carry no font dependency.
//
// Usage: node scripts/generate-brand-assets.mjs [--check]

import { writeFileSync, readFileSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join, relative } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));
const imagesDir = join(here, '..', 'docs', 'images');

export const PALETTE = {
  navy: '#0D2744',
  red: '#EA2521',
  gold: '#EFAD05',
  ivory: '#FBF2E7',
  sand: '#FCECD5',
  ink: '#0A1F36',
};

// --- mark geometry -----------------------------------------------------------
// All values are in mark-local units; the mark box is 520 x 354.
const M = {
  boxW: 520,
  boxH: 354,
  mx: 260, // mirror axis
  cy: 168, // bubble centre line
  r: 160, // bubble radius
  dx: 97, // bubble centre offset from the mirror axis
  cut: 57, // flat inner edge, measured from the bubble centre
  corner: 13, // inner edge corner radius
  barHalfLen: 112, // link bar half length
  barHalfH: 26, // link bar half height
  slotPad: 38, // clearance carved around the bar
};

const navyCx = M.mx - M.dx;
const innerX = navyCx + M.cut; // 220
const round2 = (n) => Number(n.toFixed(2));

// Inner corner: a circle of radius `corner` tangent to both the flat edge and
// the bubble outline.
const cornerCx = innerX - M.corner;
const cornerDy = Math.sqrt((M.r - M.corner) ** 2 - (cornerCx - navyCx) ** 2);
const edgeTopY = M.cy - cornerDy;
const edgeBottomY = M.cy + cornerDy;
const arcScale = M.r / (M.r - M.corner);
const arcTopX = navyCx + (cornerCx - navyCx) * arcScale;
const arcTopY = M.cy - cornerDy * arcScale;
const arcBottomY = M.cy + cornerDy * arcScale;

// Tail anchors sit on the bubble outline; the tip hangs below and outboard.
const tailBase = { dx: -108, dy: 118 };
const tailTip = { dx: -104, dy: 178 };
const tailEnd = { dx: -45, dy: 153.5 };

function mirror(x) {
  return M.mx * 2 - x;
}

function bubblePath(side) {
  const s = side === 'left' ? 1 : -1;
  const x = (v) => (side === 'left' ? v : mirror(v));
  const sweep = side === 'left' ? 0 : 1;
  return [
    `M${round2(x(innerX))} ${round2(edgeTopY)}`,
    `a${M.corner} ${M.corner} 0 0 ${sweep} ${round2(s * (arcTopX - innerX))} ${round2(arcTopY - edgeTopY)}`,
    `A${M.r} ${M.r} 0 1 ${sweep} ${round2(x(arcTopX))} ${round2(arcBottomY)}`,
    `a${M.corner} ${M.corner} 0 0 ${sweep} ${round2(s * (innerX - arcTopX))} ${round2(edgeBottomY - arcBottomY)}`,
    'Z',
  ].join('');
}

function tailPath(side) {
  const cx = side === 'left' ? navyCx : mirror(navyCx);
  const s = side === 'left' ? 1 : -1;
  const p = (d) => `${round2(cx + s * d.dx)} ${round2(M.cy + d.dy)}`;
  return `M${p(tailBase)}L${p(tailTip)}L${p(tailEnd)}Z`;
}

function barPath(side) {
  const s = side === 'left' ? -1 : 1;
  const capX = M.mx + s * (M.barHalfLen - M.barHalfH);
  const sweep = side === 'left' ? 0 : 1;
  return [
    `M${M.mx} ${M.cy - M.barHalfH}`,
    `H${round2(capX)}`,
    `a${M.barHalfH} ${M.barHalfH} 0 0 ${sweep} 0 ${M.barHalfH * 2}`,
    `H${M.mx}`,
    'Z',
  ].join('');
}

const slot = {
  x: M.mx - M.barHalfLen - M.slotPad,
  y: M.cy - M.barHalfH - M.slotPad,
  w: (M.barHalfLen + M.slotPad) * 2,
  h: (M.barHalfH + M.slotPad) * 2,
  r: M.barHalfH + M.slotPad,
};

// Gold keyline for the Agent variant. It is produced as a second, outward-grown
// copy of the same silhouette behind the fills, with the slot hole inset by the
// same amount, so the rim is exactly RIM units wide everywhere: outer edge,
// tails and slot alike.
const RIM = 6;
// The Agent variant carries its light in the keyline rather than in the colour
// blocks: a bright-to-deep gold sweep along the orbit tilt reads as a machined
// edge and keeps navy and red flat.
const GOLD_SHEEN_ID = 'dp-gold-sheen';
const goldSheen = `    <linearGradient id="${GOLD_SHEEN_ID}" gradientUnits="userSpaceOnUse" x1="20" y1="20" x2="470" y2="330" gradientTransform="rotate(-12 260 168)">
      <stop offset="0" stop-color="#FFDF88"/>
      <stop offset=".42" stop-color="${PALETTE.gold}"/>
      <stop offset="1" stop-color="#C88C06"/>
    </linearGradient>`;
const GOLD_SHEEN = `url(#${GOLD_SHEEN_ID})`;

function markBody(idSuffix, opts = {}) {
  const { node = false, rim = false } = opts;
  // Mask regions have to clear the grown rim, or the keyline is sliced off flat
  // where it leaves the mark box.
  const pad = RIM + 4;
  const region = `x="${-pad}" y="${-pad}" width="${M.boxW + pad * 2}" height="${M.boxH + pad * 2}"`;
  const cover = `<rect x="${-pad}" y="${-pad}" width="${M.boxW + pad * 2}" height="${M.boxH + pad * 2}" fill="#fff"/>`;
  const maskId = `dp-slot-${idSuffix}`;
  const rimMaskId = `dp-slot-rim-${idSuffix}`;
  const nodeMaskId = `dp-node-${idSuffix}`;
  const shapes = [
    ['left', bubblePath('left')],
    ['left', tailPath('left')],
    ['right', bubblePath('right')],
    ['right', tailPath('right')],
  ];
  const fillOf = (side) => (side === 'left' ? PALETTE.navy : PALETTE.red);
  const nodeMask = node
    ? `
    <mask id="${nodeMaskId}" maskUnits="userSpaceOnUse" ${region}>
      ${cover}
      <circle cx="${NODE.x}" cy="${NODE.y}" r="${NODE.hole}" fill="#000"/>
    </mask>`
    : '';
  const rimMask = rim
    ? `
    <mask id="${rimMaskId}" maskUnits="userSpaceOnUse" ${region}>
      ${cover}
      <rect x="${slot.x + RIM}" y="${slot.y + RIM}" width="${slot.w - RIM * 2}" height="${slot.h - RIM * 2}" rx="${slot.r - RIM}" fill="#000"/>
    </mask>`
    : '';
  const rimLayer = rim
    ? `  <g mask="url(#${rimMaskId})" fill="${GOLD_SHEEN}" stroke="${GOLD_SHEEN}" stroke-width="${RIM * 2}" stroke-linejoin="round">
${shapes.map(([, d]) => `    <path d="${d}"/>`).join('\n')}
  </g>
`
    : '';
  // The bar stays flat: it is the one element the gold node has to read against.
  const barRim = '';
  return `  <defs>
    <mask id="${maskId}" maskUnits="userSpaceOnUse" ${region}>
      ${cover}
      <rect x="${slot.x}" y="${slot.y}" width="${slot.w}" height="${slot.h}" rx="${slot.r}" fill="#000"/>
    </mask>${rimMask}${nodeMask}${rim ? `\n${goldSheen}` : ''}
  </defs>
${rimLayer}  <g mask="url(#${maskId})">
${shapes.map(([side, d]) => `    <path fill="${fillOf(side)}" d="${d}"/>`).join('\n')}
  </g>
  <g${node ? ` mask="url(#${nodeMaskId})"` : ''}>
${barRim}    <path fill="${PALETTE.navy}" d="${barPath('left')}"/>
    <path fill="${PALETTE.red}" d="${barPath('right')}"/>
  </g>`;
}

// --- agent orbit -------------------------------------------------------------
// Kept deliberately tighter than the brand board, whose orbit and ring throw the
// artwork box far past the mark and leave the lockup with a hole of whitespace.
const ORBIT = { cx: 268, cy: 132, a: 312, b: 112, tilt: -34, stroke: 11 };

function orbitPoint(tDeg) {
  const t = (tDeg * Math.PI) / 180;
  const th = (ORBIT.tilt * Math.PI) / 180;
  const x = ORBIT.a * Math.cos(t);
  const y = ORBIT.b * Math.sin(t);
  return {
    x: round2(ORBIT.cx + x * Math.cos(th) - y * Math.sin(th)),
    y: round2(ORBIT.cy + x * Math.sin(th) + y * Math.cos(th)),
  };
}

function orbitArc(fromDeg, toDeg) {
  const a = orbitPoint(fromDeg);
  const b = orbitPoint(toDeg);
  const large = Math.abs(toDeg - fromDeg) > 180 ? 1 : 0;
  const sweep = toDeg > fromDeg ? 1 : 0;
  return `M${a.x} ${a.y}A${ORBIT.a} ${ORBIT.b} ${ORBIT.tilt} ${large} ${sweep} ${b.x} ${b.y}`;
}

const RING = { t: -46, outer: 27, stroke: 17 };
// The node is punched clean through the link bar: a transparent hole, a gold rim
// sunk into it, and a hollow centre.
const NODE = { x: M.mx, y: M.cy, hole: 32, rim: 30, rimStroke: 13 };

// The far side of the orbit runs behind the bubbles; only the gaps between them
// are drawn, as dashes, so the ring reads as a node on a real orbit.
function insideMark(p) {
  const inSlot =
    p.x > slot.x + slot.r
      ? p.x < slot.x + slot.w - slot.r
        ? p.y > slot.y && p.y < slot.y + slot.h
        : Math.hypot(p.x - (slot.x + slot.w - slot.r), p.y - M.cy) < slot.r
      : Math.hypot(p.x - (slot.x + slot.r), p.y - M.cy) < slot.r;
  if (inSlot) return false;
  const inBubble = (cx, inner, outward) =>
    Math.hypot(p.x - cx, p.y - M.cy) < M.r && (outward > 0 ? p.x < inner : p.x > inner);
  return (
    inBubble(navyCx, innerX, 1) || inBubble(mirror(navyCx), mirror(innerX), -1)
  );
}

function occludedRanges(fromDeg, toDeg) {
  const step = fromDeg < toDeg ? 1 : -1;
  const spans = [];
  let open = null;
  for (let t = fromDeg; step > 0 ? t <= toDeg : t >= toDeg; t += step) {
    const visible = !insideMark(orbitPoint(t));
    if (visible && open === null) open = t;
    if (!visible && open !== null) {
      if (Math.abs(t - open) > 5) spans.push([open, t - step]);
      open = null;
    }
  }
  if (open !== null && Math.abs(toDeg - open) > 5) spans.push([open, toDeg]);
  return spans;
}

function agentOrbit() {
  const ring = orbitPoint(RING.t);
  const behind = occludedRanges(-64, -176)
    .map((span) => `    <path d="${orbitArc(span[0], span[1])}" stroke-dasharray="3 28"/>`)
    .join('\n');
  const box = AGENT_BOX;
  return `  <defs>
    <mask id="dp-orbit" maskUnits="userSpaceOnUse" x="${box.x}" y="${box.y}" width="${box.w}" height="${box.h}">
      <rect x="${box.x}" y="${box.y}" width="${box.w}" height="${box.h}" fill="#fff"/>
      <circle cx="${ring.x}" cy="${ring.y}" r="${RING.outer}" fill="#000"/>
    </mask>
  </defs>
  <g mask="url(#dp-orbit)" fill="none" stroke="${GOLD_SHEEN}" stroke-width="${ORBIT.stroke}" stroke-linecap="round">
    <path d="${orbitArc(186, -48)}"/>
${behind}
  </g>
  <circle cx="${ring.x}" cy="${ring.y}" r="${RING.outer - RING.stroke / 2}" fill="none" stroke="${GOLD_SHEEN}" stroke-width="${RING.stroke}"/>
  <circle cx="${NODE.x}" cy="${NODE.y}" r="${NODE.rim - NODE.rimStroke / 2}" fill="none" stroke="${GOLD_SHEEN}" stroke-width="${NODE.rimStroke}"/>`;
}

// The agent viewBox is derived from what is actually drawn, so the orbit never
// pads the artwork with empty margin.
function agentViewBox() {
  const pad = 6;
  const xs = [-RIM, M.boxW + RIM];
  const ys = [-RIM, M.boxH + RIM];
  const ranges = [[186, -48], ...occludedRanges(-64, -176)];
  for (const [from, to] of ranges) {
    const step = from < to ? 1 : -1;
    for (let t = from; step > 0 ? t <= to : t >= to; t += step) {
      const p = orbitPoint(t);
      xs.push(p.x - ORBIT.stroke / 2, p.x + ORBIT.stroke / 2);
      ys.push(p.y - ORBIT.stroke / 2, p.y + ORBIT.stroke / 2);
    }
  }
  const ring = orbitPoint(RING.t);
  xs.push(ring.x - RING.outer, ring.x + RING.outer);
  ys.push(ring.y - RING.outer, ring.y + RING.outer);
  const x = Math.floor(Math.min(...xs)) - pad;
  const y = Math.floor(Math.min(...ys)) - pad;
  return {
    x,
    y,
    w: Math.ceil(Math.max(...xs)) + pad - x,
    h: Math.ceil(Math.max(...ys)) + pad - y,
  };
}

const AGENT_BOX = agentViewBox();

// --- wordmarks ---------------------------------------------------------------
// Tomorrow Bold outlines, normalised to cap-height 100 with the baseline on
// y = 0, so callers scale by (target cap height / 100) and never need the source
// font. See scripts/generate-brand-wordmarks.mjs for the typeface rationale.
const WORDMARK_SOURCE = JSON.parse(readFileSync(join(here, 'brand-wordmarks.json'), 'utf8'));
const WORD_CAP = WORDMARK_SOURCE.capHeight;
const WORDMARK = Object.fromEntries(Object.entries(WORDMARK_SOURCE.marks).map(([k, v]) => [k, v.d]));
const WORD_WIDTH = Object.fromEntries(Object.entries(WORDMARK_SOURCE.marks).map(([k, v]) => [k, v.width]));

const round = (v) => Number(v.toFixed(2));

function svg(attrs, body) {
  return `<svg xmlns="http://www.w3.org/2000/svg" ${attrs}>\n${body}\n</svg>\n`;
}

function label(text, x, y, size, color, tracking) {
  return `  <text x="${x}" y="${y}" fill="${color}" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="${size}" font-weight="600" letter-spacing="${tracking}">${text}</text>`;
}

// --- assets ------------------------------------------------------------------
function imMark() {
  return svg(
    `viewBox="0 0 ${M.boxW} ${M.boxH}" role="img" aria-labelledby="dp-im-title dp-im-desc"`,
    `  <title id="dp-im-title">Dipole IM</title>
  <desc id="dp-im-desc">Two facing conversation bubbles, navy and signal red, joined by a two-tone link bar.</desc>
${markBody('im')}`
  );
}

function agentMark() {
  return svg(
    `viewBox="${AGENT_BOX.x} ${AGENT_BOX.y} ${AGENT_BOX.w} ${AGENT_BOX.h}" role="img" aria-labelledby="dp-ag-title dp-ag-desc"`,
    `  <title id="dp-ag-title">Dipole Agent</title>
  <desc id="dp-ag-desc">The Dipole IM mark with a gold orbit and a knocked-out node, standing for governed Agent work.</desc>
${markBody('agent', { node: true, rim: true })}
${agentOrbit()}`
  );
}

function favicon() {
  const pad = 44;
  const scale = (256 - pad * 2) / M.boxW;
  const ty = (256 - M.boxH * scale) / 2;
  return svg(
    `viewBox="0 0 256 256" role="img" aria-labelledby="dp-fav-title"`,
    `  <title id="dp-fav-title">Dipole</title>
  <rect width="256" height="256" rx="56" fill="${PALETTE.ivory}"/>
  <g transform="translate(${pad} ${round2(ty)}) scale(${round2(scale)})">
${markBody('fav')}
  </g>`
  );
}

function lockup() {
  // The lockup measures itself: mark bounds and wordmark advance widths decide
  // the column positions, so a font or geometry change can never push artwork
  // past the panel edge.
  const W = 1040;
  const H = 224;
  const margin = 48;
  const markScale = 0.3;
  const capHeight = 30;
  const wordScale = capHeight / WORD_CAP;
  const gap = 26; // between a mark and its wordmark
  const axis = 94; // shared centre line for marks and wordmarks
  const cols = [
    { id: 'im', orbit: false, word: WORDMARK.im, width: WORD_WIDTH.im, tag: 'IM DATA PLANE' },
    { id: 'agent', orbit: true, word: WORDMARK.agent, width: WORD_WIDTH.agent, tag: 'AGENT CONTROL PLANE' },
  ];
  for (const col of cols) {
    col.box = col.orbit ? AGENT_BOX : { x: 0, y: 0, w: M.boxW, h: M.boxH };
    col.markW = col.box.w * markScale;
    col.wordW = col.width * wordScale;
    col.w = col.markW + gap + col.wordW;
  }
  const slack = W - margin * 2 - cols[0].w - cols[1].w;
  const divider = margin + cols[0].w + slack / 2;
  cols[0].x = margin;
  cols[1].x = divider + slack / 2;

  const body = cols
    .map((col) => {
      // Translate so the mark's own bounding box starts at col.x, and its
      // capsule centre line sits on the shared axis.
      const mx = col.x - col.box.x * markScale;
      const my = axis - M.cy * markScale;
      const textX = col.x + col.markW + gap;
      return `  <g transform="translate(${round(mx)} ${round(my)}) scale(${markScale})">
${markBody(col.id, { node: col.orbit, rim: col.orbit })}${col.orbit ? `\n${agentOrbit()}` : ''}
  </g>
  <g transform="translate(${round(textX)} ${axis + capHeight / 2}) scale(${round(wordScale)})" fill="${PALETTE.navy}">
    <path d="${col.word}"/>
  </g>
${label(col.tag, round(textX + 2), axis + capHeight / 2 + 24, 11, '#5C6675', '2.6')}`;
    })
    .join('\n');
  const swatchKeys = [
    ['navy', PALETTE.navy],
    ['red', PALETTE.red],
    ['gold', PALETTE.gold],
    ['ivory', PALETTE.ivory],
  ];
  const swatches = swatchKeys
    .map(([name, hex], i) => {
      const x = margin + i * 152;
      return `  <rect x="${x}" y="${181}" width="14" height="14" rx="3" fill="${hex}" stroke="#DFD2BB"/>
${label(`${name.toUpperCase()} ${hex}`, x + 22, 192, 10, '#5C6675', '0.6')}`;
    })
    .join('\n');
  return svg(
    `viewBox="0 0 ${W} ${H}" role="img" aria-labelledby="dp-lock-title dp-lock-desc"`,
    `  <title id="dp-lock-title">Dipole IM and Dipole Agent</title>
  <desc id="dp-lock-desc">The Dipole V3 dual-product lockup: navy and red conversation poles for IM, a gold orbit for governed Agent work.</desc>
  <rect width="${W}" height="${H}" rx="18" fill="${PALETTE.ivory}"/>
${body}
  <path d="M${round(divider)} 38V150" stroke="#E2D6C0" stroke-width="1.5"/>
  <path d="M${margin} 160H${W - margin}" stroke="#E2D6C0" stroke-width="1.5"/>
${swatches}
  <text x="${W - margin}" y="192" text-anchor="end" fill="#5C6675" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="10" font-weight="600" letter-spacing="1.6">DIPOLE BRAND V3</text>`
  );
}

const assets = {
  'dipole-v3-im.svg': imMark(),
  'dipole-v3-agent.svg': agentMark(),
  'dipole-v3-favicon.svg': favicon(),
  'dipole-v3-brand-lockup.svg': lockup(),
};

// The web client gets its own copies so the frontend build never reaches outside
// its root, and so the tab icon and login marks cannot drift from docs/images.
const frontend = join(here, '..', 'frontend');
const publicCopies = {
  [join(frontend, 'public', 'dipole-v3-favicon.svg')]: assets['dipole-v3-favicon.svg'],
  [join(frontend, 'src', 'assets', 'brand', 'dipole-v3-im.svg')]: assets['dipole-v3-im.svg'],
  [join(frontend, 'src', 'assets', 'brand', 'dipole-v3-agent.svg')]: assets['dipole-v3-agent.svg'],
};

const check = process.argv.includes('--check');
let drift = 0;
const targets = Object.entries(assets)
  .map(([name, content]) => [join(imagesDir, name), content])
  .concat(Object.entries(publicCopies));
for (const [target, content] of targets) {
  const name = relative(join(here, '..'), target);
  if (check) {
    const current = existsSync(target) ? readFileSync(target, 'utf8') : '';
    if (current !== content) {
      drift += 1;
      console.error(`drift: ${name} does not match scripts/generate-brand-assets.mjs`);
    }
  } else {
    writeFileSync(target, content);
    console.log(`wrote ${name}`);
  }
}
if (check) {
  if (drift > 0) {
    console.error(`\n${drift} brand asset(s) out of date. Run: node scripts/generate-brand-assets.mjs`);
    process.exit(1);
  }
  console.log('brand assets match the generator');
}
