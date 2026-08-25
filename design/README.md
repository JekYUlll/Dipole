# Dipole Frontend Design

`dipole-ui.pen` is the canonical editable design for the Dipole web client. Approved overview exports live in `exports/`; implementation details and visual decisions are recorded in `DESIGN-CHANGELOG.md`.

## Update Workflow

1. Verify `pen status` and `pen version`.
2. Update the canonical file with `pen --in design/dipole-ui.pen --out design/dipole-ui.pen`.
3. Export a 2x review image into `design/exports/`.
4. Record changed frames, components, tokens, and states in `DESIGN-CHANGELOG.md`.
5. Implement the approved design in Vue and run type, E2E, visual, responsive, and accessibility checks.

Do not create a replacement `.pen` for routine feature work. Iterate on the canonical file so components and product history remain connected.

## F1 Frames

| Frame | Review export |
| --- | --- |
| Foundations + Components | `exports/foundations.png` |
| Login Desktop | `exports/login-desktop.png` |
| Login Mobile | `exports/login-mobile.png` |
| Chat Desktop | `exports/chat-desktop.png` |
| Chat Mobile | `exports/chat-mobile.png` |
| Design Review Checklist | `exports/review-checklist.png` |
| Full canvas overview | `exports/dipole-ui-overview.png` |
