# Dipole V3 UI Design Brief

## Scope

Incrementally add a new isolated cluster named `Brand / V3 / Dipole IM & Agent` to the existing canonical `design/dipole-ui.pen` document. Preserve every existing frame, component, and annotation. This is a design-system and primary-flow addition, not a reset of earlier work.

## Brand Direction

Use the supplied Dipole V3 language as the source of truth:

- `Navy #0B2A4A`: trustworthy IM infrastructure and navigation.
- `Signal red #F2262A`: message energy, unread state, and primary action.
- `Orbit gold #F4B000`: Agent state, durable task progress, and intelligent collaboration.
- `Ivory #F8F1E4`: warm canvas and paper-like secondary surfaces.
- `Ink #092545`: readable long-form text.

The mark is formed from two opposing conversation poles: a navy left form and a red right form joined through a small white capsule. The Agent version adds a fine gold orbital arc and a small gold node. The visual language should be crisp, editorial, and warm. Avoid generic AI gradients, soft purple palettes, or oversized rounded blobs.

## Add These Frames

### `Brand/V3/Foundations`

Show the palette, semantic colors, type hierarchy, spacing scale, radius rules, status chips, button variants, input states, and both IM and Agent mark variants. Include minimum logo size and clear-space notes.

### `Brand/V3/Login/Desktop` and `Brand/V3/Login/Mobile`

Design a focused sign-in experience. Use an ivory canvas with a subtle, sparse gold orbit motif. Give the identity panel enough space to introduce Dipole IM and Dipole Agent. The credential form should remain calm, accessible, and practical, with explicit focus, error, and loading states.

### `Brand/V3/Chat/Desktop` and `Brand/V3/Chat/Mobile`

Design the primary IM shell: a restrained navy product rail, an ivory conversation rail, and a clean white conversation canvas. Red communicates unread/presence and the compose primary action. Gold is reserved for Agent status and task affordances. Include a compact connection state and responsive mobile navigation. Use realistic Chinese content and avoid claims of uncontrolled Agent authority.

### `Brand/V3/Agent Task/Desktop` and `Brand/V3/Agent Task/Mobile`

Design the durable task entry and timeline shell. Make task status, read-only scope, tool approval, and waiting-for-user state immediately understandable. Use gold sparingly for Agent identity/progress, navy for trusted controls, and red only for risk or user-attention actions.

## Product Safety Language

The interface should make authorization boundaries visible. Include text such as "只读范围" and "需要确认后执行" where appropriate. Do not imply that the Agent can write to a group or access a conversation without a user approval path.

## Deliverable

Create an overview suitable for design review with desktop and mobile compositions in one coherent V3 cluster. Keep labels in Chinese and make component intent inspectable for the Vue implementation that follows.
