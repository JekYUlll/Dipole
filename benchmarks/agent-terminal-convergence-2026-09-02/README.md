# Agent Terminal Convergence Receipt

## Scope

This receipt records one disposable, loopback-only Remote GPU observation for
`agent-runtime@d591bc7592b3974c6ec33425371f66fd9d3e29ea`. The isolated Compose
project exercised Kafka event admission, Core mTLS capability access, Temporal
read-shadow execution, MySQL audit projection, and the Agent review-pack
exporter.

The candidate bound Gateway to `127.0.0.1:18118`. It did not alter host
networking, the Docker daemon, or another Compose project.

## Observed Result

[`review-pack.json`](review-pack.json) is a low-sensitivity export created by
the dedicated read-only evaluator account. It records one trace-bound
execution where all terminal projections converged:

- Event ledger: `completed`.
- Durable Task and Temporal workflow: `completed`.
- Persistent Agent Run: `completed`.
- Output: exactly one `conversation_digest` Artifact.
- Capability trajectory: authorized `conversation.list`; the dependent
  `conversation.read` was explicitly `not_required` because no conversation
  was discovered.
- Model route: two completed `deepseek/deepseek-v4-flash` calls with complete
  token and latency metering.

The receipt verifies the Core fix that converges the parent Task when a Run
reaches a terminal state. It also confirms the repository smoke assertion now
waits for `agent_tasks.status=completed` rather than accepting a stale
`running` Task.

## Limits

This is an isolated `N=1` regression receipt. It does not establish an Agent
task success rate, shared-environment reliability, public experience URL,
latency SLO, active authority, message write safety, MCP behavior, or a
candidate promotion decision. The sample did not perform an actual
`conversation.read`, because discovery returned no conversation.

A claimable evaluation window still requires a fixed, independently reviewed
multi-sample manifest set and the existing Shadow Eval gates.
