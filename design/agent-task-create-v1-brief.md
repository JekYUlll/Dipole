# Agent Task Create v1 Pencil Brief

Extend `design/dipole-ui.pen` incrementally. Preserve every existing frame and component.

Add these named frames using existing foundations and tokens:

- `Agent Task Create/Desktop`: an authenticated, focused composer for an interactive Agent Task. Show a concise goal field, a visible local request identity label, a clear statement that the Agent has read-only conversation access, and a submit action that leads to the existing Task Timeline.
- `Agent Task Create/Mobile`: the same flow in a narrow single-column layout without horizontal overflow.
- `Agent Task Create/State Matrix`: idle, validation error, submitting, accepted/redirecting, and unavailable states.

Add named reusable components only where useful: `Component/Agent Task Goal Field`, `Component/Agent Task Request Badge`, and `Component/Agent Task Submit State`.

Keep the Signal Link visual language: light canvas, deep rail, green accent, orange event pulse, Manrope display, Noto Sans SC body, and existing spacing/radius variables. The surface must communicate safety: it submits a user goal, does not expose or edit tenant, principal, Agent identity, tools, memory, credentials, raw events, or model prompts. Validation and unavailable states must not claim a Task was created. Do not add write Capability controls, Agent Definition editing, or Runtime activation controls.
