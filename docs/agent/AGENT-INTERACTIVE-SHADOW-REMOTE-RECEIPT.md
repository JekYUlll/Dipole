# Agent Interactive Shadow Remote Receipt

## Scope

This receipt records a development-only acceptance run on 2026-09-01. The
candidate Compose project ran on Remote GPU with its Gateway bound to
`127.0.0.1:18095`; no public port was opened and no shared `8080` service was
changed.

The loaded profiles were `remote-gpu-mysql-aio-compat`,
`agent-temporal-read-shadow`, `agent-ai-sdk-shadow`, and
`agent-interactive-shadow`. Agent Runtime remained in `shadow` and
`read_shadow` modes. MCP, external MCP, Memory promotion, active authority,
and write capabilities remained disabled.

## Acceptance

Two temporary users were registered through the public Gateway API. Each user
submitted the same read-only goal: list accessible conversations and summarize
the result.

| Check | Result |
| --- | --- |
| Gateway health on candidate loopback port | passed |
| JWT-authenticated `POST /api/v1/agent/tasks` | passed |
| Durable Task terminal state | `completed` for both runs |
| `GET /api/v1/agent/tasks/{id}/timeline?limit=2` | passed |
| Continuation cursor and second Timeline page | passed; two events on each page |

The final candidate Gateway image was built from source revision `4ab924b87`
and tagged `dipole-gateway:agent-timeline-4ab924b8`. The existing Core and
Agent candidate images were compatible with that Gateway for this read-only
flow. This is a mixed-version development compatibility result, not a
same-revision release receipt.

## Data Handling

Temporary passwords and JWTs existed only in the probe process. This receipt
does not retain user identities, tokens, task IDs, request IDs, conversation
content, model output, or provider credentials.

## Limits And Follow-up

The result proves the interactive Task admission, Temporal execution, terminal
projection, and paginated Timeline read path in an isolated Remote GPU
candidate. It does not establish a task success-rate metric, active write
authority, a production deployment, or a public experience URL.

Before citing the flow as a release acceptance result, rebuild Core, Gateway,
and Agent from one revision, archive a repeatable low-sensitive receipt, and
run a controlled observation window with the approved evaluation manifests.
