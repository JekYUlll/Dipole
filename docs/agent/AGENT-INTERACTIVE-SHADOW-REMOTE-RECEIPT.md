# Agent Interactive Shadow Remote Receipt

## Scope

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
