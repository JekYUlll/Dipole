# Dipole Brand Assets

These SVG assets are the source of truth for the repository identity. They are dependency-free so GitHub, package registries and documentation render them consistently.

The historical blue PNG logo has been retired. New documentation must use one of
the SVG assets below so the repository keeps a consistent visual identity.

| Asset | Intended use |
| --- | --- |
| `dipole-wordmark.svg` | Repository header and landing pages. |
| `dipole-mark.svg` | Square application icon, favicons and compact product entry points. |
| `dipole-im-mark.svg` | IM-focused materials and architecture documents. |
| `dipole-agent-mark.svg` | Agent Runtime-focused materials and architecture documents. |
| `dipole-v3-im.svg` | Symmetric V3 IM mark for product entry points. |
| `dipole-v3-im-traced.svg` | PNG-derived Dipole IM logo, generated with VTracer from `LOGO_V3.png`. |
| `dipole-v3-im-mark-traced.svg` | Historical compact IM trace, retained for source-asset continuity. |
| `dipole-v3-agent-traced.svg` | PNG-derived Dipole Agent logo, generated with VTracer from `LOGO_V3.png`. |
| `dipole-v3-agent-mark-traced.svg` | PNG-derived compact Agent mark for narrow control rails, generated with VTracer from the Agent crop in `LOGO_V3.png`. |

The `dipole-v3-*-traced.svg` files preserve the PNG concept artwork as
transparent vector paths. The canvas color is removed before tracing, and the
crop coordinates intentionally exclude the concept heading and palette
swatches. Recreate them with `scripts/trace-brand-assets.sh`; the crop
coordinates and VTracer parameters are kept in that script so later revisions
remain reproducible. The generated files contain SVG paths only and do not
embed the source PNG. Use the symmetric `dipole-v3-im.svg` for all user-facing
IM pages. The traced variants remain available as reproducible source assets.

Keep the V3 navy and signal-red palette aligned across new assets. The Agent
mark represents governed tasks and capabilities only; it must not imply
privileged capabilities are enabled by default.
