# Agent Interactive Active Current Receipt

## Public Gateway Acceptance (2026-09-03)

This development-only Remote GPU candidate runs the `649cf110` source
revision in an isolated Compose project. Gateway is published on the host at
port `18121`; Core, Gateway, Message, Sync, Kafka, Temporal, MySQL, Redis,
MinIO, and the TypeScript Agent Runtime were all healthy during the acceptance
run.

The runtime used the explicit `interactive_active` profile. It enabled the
authenticated Agent Task control API and the narrow `/send <content>` path for
the owner's direct Agent conversation. External MCP, Artifact access, OAuth
callback, Memory writes, and all other write surfaces remained disabled.

| Check | Result |
| --- | --- |
| Public Web application and Agent Task route | `GET /app/` and `/app/agent/tasks/new` returned `200` |
| Authenticated runtime status | `active`, Task control enabled, interactive message writes enabled |
| Approved `/send` Task | `waiting_approval` -> `completed` |
| Approved Task Timeline | five events returned through the public Gateway |
| Approved durable effects | one completed Tool Invocation, one consumed approval, one Agent system message |
| Denied `/send` Task | `waiting_approval` -> `cancelled` |
| Denied durable effects | no additional Tool Invocation, message, or approval consumption |

The acceptance used a temporary owner and a short-lived, exact-conversation
promotion grant. The grant was revoked after each path. Temporary identities,
passwords, JWTs, task identifiers, message content, model credentials, and
runtime secrets are intentionally absent from this receipt.

## Experience Boundary

The server-side experience is available through the Gateway API and the
candidate Web application is reachable from the Remote GPU host. A direct
probe from outside the host returned an upstream `404` while the host's own
public-address probe returned `200`; this points to a network layer outside
the Compose project. No firewall, route, Docker daemon, or cloud network
change was made. A user-facing URL must be verified after the external routing
owner is identified.

Current Vue Agent routes are guarded by build-time `VITE_AGENT_*_ENABLED`
flags. Until the independently maintained frontend build enables the create,
approval, and Timeline flags and passes browser tests, this receipt
demonstrates the authenticated API and durable execution path, not a completed
browser Human-in-the-loop release.

The first task read may briefly return `404` while the asynchronous admission
record is being projected. Browser code must retry only the just-created,
owner-scoped Task for a finite window before presenting an error.

## Scope

This receipt supports development acceptance of the narrow interactive write
path. It does not establish production authorization, external MCP
connectivity, a model-quality metric, task success-rate statistics, capacity,
or a shared-environment release.
