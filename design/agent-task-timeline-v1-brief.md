# Agent Task Timeline v1 Pencil Brief

Extend `design/dipole-ui.pen` without rebuilding existing frames.

Add these named frames using the existing foundations and tokens:

- `Agent Timeline/Desktop/Events`: a focused task execution history with task identity, current revision, event sequence, event kind, status, time, capability label, and a low-sensitivity provenance note.
- `Agent Timeline/Mobile/Events`: the same content in a narrow single-column layout with readable event grouping and no horizontal overflow.
- `Agent Timeline/State Matrix`: loading, empty, unavailable/retry, and paginated older-events states.

Keep the visual language consistent with the existing light canvas, deep rail, green accent, Manrope display, Noto Sans SC body, and existing radius/spacing variables. Make the trust boundary visible: event content is metadata only, untrusted evidence is labeled, and unavailable states must not imply that history was loaded. Reuse or add named components for timeline event, revision badge, provenance label, and unavailable state. Do not add controls for editing, deleting, or executing a task.
