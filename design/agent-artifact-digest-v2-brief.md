# Agent Artifact Digest Reader v2 Pencil Brief

Extend `design/dipole-ui.pen` incrementally. Preserve every existing frame, component, token, and export.

Add these named frames using the existing Signal Link foundations:

1. `Agent Artifact/Desktop/Digest Reader`
2. `Agent Artifact/Mobile/Digest Reader`
3. `Agent Artifact/Digest State Matrix`

Add named reusable components:

1. `Component/Agent Artifact Digest Body`
2. `Component/Agent Artifact Digest Status`

The page evolves the existing owner-scoped metadata surface. It may render the text body only for a verified `conversation_digest` with `text/markdown`; title, content address, Task/Run and created time remain visible above the reader. The digest body uses a calm paper-like reading panel with visible line wrapping, no download affordance, no HTML preview, and no editable controls.

The state matrix must cover:

- metadata loading;
- verified digest ready;
- digest body loading while metadata remains visible;
- digest unavailable with a retry action and metadata retained;
- unsupported Artifact type, where metadata remains visible and the reader explains that no displayable digest is available.

Desktop should keep a focused two-column reading composition with metadata/integrity context beside the digest. Mobile is a single column without horizontal overflow. Reuse the existing light canvas, deep rail, green accent, orange warning, Manrope display, Noto Sans SC body, Geist Mono metadata, and existing spacing/radius tokens.

The design must keep object keys, metadata JSON, public URLs, generic downloads, write actions, owner/tenant identifiers, raw prompts, Tool arguments, and Runtime control out of the browser surface. Do not imply that viewing a digest activates the Agent Runtime or grants any write capability.

Export the new frames at 2x into `design/exports/agent-artifact-v2/` and export the full review canvas to `design/exports/agent-artifact-v2/overview.png`. Keep all node names readable and leave no placeholder nodes.
