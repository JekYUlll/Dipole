# Dipole V3 Brand Kit

`design/brand/` is the versioned source of truth for the Dipole visual identity. It packages the V3 dual-pole mark for IM, the governed Agent orbit, approved color roles, and practical usage rules.

## Asset Layout

| Location | Purpose |
| --- | --- |
| `source/` | Canonical full-color vector assets. Edit these only through a reviewed brand change. |
| `variants/` | Approved one-color, reverse, and favicon forms for constrained surfaces. |
| `reference/` | The approved V3 raster reference board. It is context for review, not a runtime asset. |
| `manifest.json` | Machine-readable asset inventory, public-copy mapping, and palette. |
| `../../docs/images/` | Published copies consumed by README and Vue component imports. The brand asset gate requires them to match `source/`. |
| `../../frontend/public/` | Browser-shell assets, currently the V3 favicon. The brand asset gate requires it to match the approved variant. |

## Approved Assets

| Asset | Use |
| --- | --- |
| `source/dipole-v3-im.svg` | Primary Dipole IM product mark on paper, ivory, or white surfaces. |
| `source/dipole-v3-agent.svg` | Dipole Agent mark where durable, governed Agent work is in scope. |
| `source/dipole-v3-brand-lockup.svg` | README, overview, and product-family presentation. |
| `variants/dipole-v3-*-reverse.svg` | Navy product rail, footer, or other dark surfaces. |
| `variants/dipole-v3-*-mono-navy.svg` | Single-ink print or vendor-restricted output. |
| `variants/dipole-v3-favicon.svg` | Browser, app, and compact square contexts. |

## Palette And Rules

| Role | Value | Use |
| --- | --- | --- |
| Navy | `#0B2A4A` | Trusted IM navigation and controls. |
| Ink | `#092545` | Long-form readable text. |
| Signal red | `#F2262A` | Message energy, unread state, and primary action. |
| Orbit gold | `#F4B000` | Agent identity, durable work, and governed progress. |
| Ivory | `#F8F1E4` | Warm secondary canvas and mark capsule. |
| Paper | `#FFFDF8` | Primary reading surface. |

- Keep a clear space of at least one quarter of the mark width on every side.
- Use the IM mark from 20 px and the Agent mark from 24 px; use the favicon below those thresholds.
- Preserve the navy/red relationship. The gold orbit only belongs to Agent contexts.
- Do not redraw the pole geometry, add effects, place the primary mark over busy imagery, or use the raster reference as a product icon.

## Change Procedure

1. Update the canonical SVG under `source/` and any approved variants.
2. Update every matching published or runtime file declared in `manifest.json` in the same change.
3. Update `manifest.json`, this guide, `design/DESIGN-CHANGELOG.md`, and the relevant Pencil frame when the visual contract changes.
4. Run `cd frontend && npm run test:design`.

The brand kit governs shared identity. Page composition, responsive behavior, and component states remain in [`../dipole-ui.pen`](../dipole-ui.pen).
