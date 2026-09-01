# Dipole Brand Assets

`dipole-brand-v3.png` is the user-supplied V3 brand board and remains the visual
reference of record. Every SVG in this directory is generated from
`scripts/generate-brand-assets.mjs`, which holds the palette and the mark
geometry as one set of constants.

Regenerate after any change to that script, and never hand-edit the SVGs:

```bash
node scripts/generate-brand-assets.mjs          # rewrite the assets
node scripts/generate-brand-assets.mjs --check  # fail if an asset drifted
```

`npm run test:brand` in `frontend/` runs the drift check. The generator also
mirrors the favicon into `frontend/public/` and both marks into
`frontend/src/assets/brand/`, so the web client never reaches outside its own
root for brand artwork.

| Asset | Intended use |
| --- | --- |
| `dipole-v3-brand-lockup.svg` | README header and canonical IM/Agent lockup with wordmarks and palette. |
| `dipole-v3-im.svg` | IM application icon, login entry points and IM-focused materials. |
| `dipole-v3-agent.svg` | Agent Runtime icon, durable-task entry points and Agent-focused materials. |
| `dipole-v3-favicon.svg` | Browser tab icon; mirrored into `frontend/public/` by the generator. |
| `dipole-brand-v3.png` | Visual reference for the marks above. |

## Construction

Both marks share one geometry: two opposing speech bubbles, navy on the left and
red on the right, each a 160-radius disc with a straight inner edge and a
downward tail, bridged by an ivory capsule slot knocked out of both bubbles. The
capsule is split navy/red at the centre line, so the bridge reads as one
conductor between two poles.

The Agent mark adds, and only adds, a gold layer over that identical body:

- a 6-unit gold keyline around the bubble silhouette and the slot,
- a tilted elliptical orbit with a hollow ring and three tapering trail dots,
- a hollow gold node punched through the centre of the bridge capsule.

The gold carries a bright-to-deep gradient along the orbit tilt so the Agent
variant reads as machined metal, while navy and red stay flat. Do not add
gradients to the colour blocks themselves; the crispness of the flat fills is
what distinguishes V3 from the retired identity.

## Typography

The logotype is **Goldman Bold**, a wide blocky face whose squared counters and
heavy uniform stems read as machined hardware and deliberately contrast the
mark's discs. Softer geometric faces made the lockup look generic, so do not
substitute one.

It ships as SVG outlines, not a `font-family` reference, so the assets render
identically on GitHub, in the product and in exported images. Regenerate the
outlines with `scripts/generate-brand-wordmarks.mjs`; that script documents the
one-off tooling install and normalises every logotype to cap-height 100 with the
baseline on `y = 0`.

The lockup then fits itself to that logotype. Marks are fixed width, so the
panel takes the largest cap height that still clears its gutters, capped at the
size that reads well beside a mark. Goldman is roughly 40% wider than a normal
grotesque and therefore sets smaller rather than colliding with the divider; a
typeface change can never push artwork past the panel edge.

Supporting labels stay in a system monospace stack, letterspaced and uppercase.
They are the data-plane voice of the system and need no embedded font.

## Rules

- Keep the opposing navy/red poles, the ivory bridge capsule and the warm ivory
  canvas aligned across new assets.
- Gold belongs to the Agent variant only. It signals governed capability, not
  privilege: the Agent mark must never imply privileged capabilities are enabled
  by default.
- New UI and documentation work aligns with V3 rather than reintroducing the
  retired teal/blue identity (`#07c160` and friends are gone).
