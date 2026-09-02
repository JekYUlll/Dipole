# Agent Interactive Shadow Remote Receipt

## Same-Revision Acceptance (2026-09-02)

This development-only Remote GPU run used a fresh Compose project with its
Gateway bound to `127.0.0.1:18113`. Every Core, Gateway, Message, Sync,
Search, migration, repair, and Agent image carried source revision
`d7fee99afc71ebcae6ffac68efb43e74f3655e1d` and
`io.dipole.source.dirty=false`. No shared port, container, volume, network, or
Docker daemon configuration was changed.

The run loaded `remote-gpu-mysql-aio-compat`, `agent-temporal-read-shadow`,
`agent-ai-sdk-shadow`, and `agent-interactive-shadow`. Runtime remained in
`shadow` and `read_shadow`; MCP, external MCP, Memory promotion, active
authority, and write capabilities remained disabled.

| Check | Result |
| --- | --- |
| Candidate image provenance | same clean revision for all participating services |
| Gateway health on loopback candidate port | passed |
| JWT-authenticated `POST /api/v1/agent/tasks` | `202` |
| Durable Task terminal state | `completed` |
| First Timeline page (`limit=2`) | `200`, two events |
| Cursor continuation page | `200`, two events |

The probe used one temporary user and a read-only conversation-listing goal.
Passwords, JWTs, user identifiers, task identifiers, request identifiers,
conversation content, model output, and provider credentials were retained
only in the process and were not archived.

This proves the isolated same-revision interactive read-shadow path. It does
not establish a task success-rate metric, active write authority, a production
deployment, a public experience URL, or a shared-environment observation
window.

## Mixed-Version Acceptance (2026-09-01)

This receipt records a development-only acceptance run on 2026-09-01. The
isolated Compose project ran on Remote GPU from source revision `a273bae2`.
Its Gateway was bound to `18099`; no shared `8080` service, Docker daemon, or
host networking configuration was changed.

The loaded profiles were `remote-gpu-mysql-aio-compat`,
`agent-temporal-read-shadow`, `agent-ai-sdk-shadow`,
`agent-interactive-shadow`, and `agent-deepseek-v4-flash-shadow`. Agent
Runtime remained in `shadow` and `read_shadow` modes. The DeepSeek profile
used JSON-text output with reasoning disabled. MCP, external MCP, Memory
promotion, active authority, and write capabilities remained disabled.

## Acceptance

A temporary user was registered through the candidate Gateway API and
submitted a read-only summary goal.

| Check | Result |
| --- | --- |
| Candidate Web application | `GET /app/` returned `200` |
| JWT-authenticated `POST /api/v1/agent/tasks` | passed |
| Durable Task terminal state | `completed` |
| Task Timeline | five events returned |
| Digest Artifact | one `conversation_digest` Artifact event returned |

Core, Gateway, Message, Sync, and Agent candidate images were built from the
same source revision. This is a development acceptance result, not a
production release or capacity result.

## Data Handling

Temporary passwords and JWTs existed only in the probe process. This receipt
does not retain user identities, tokens, task IDs, request IDs, conversation
content, model output, or provider credentials.

## Limits And Follow-up

The result proves interactive Task admission, Temporal execution, terminal
projection, Timeline output, and digest Artifact creation in an isolated
Remote GPU candidate. It does not establish a task success-rate metric, active
write authority, production deployment, or capacity claim.

Before citing the flow as a release acceptance result, archive a repeatable
low-sensitive receipt and run a controlled observation window with approved
evaluation manifests.
