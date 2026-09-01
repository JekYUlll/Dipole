# Interactive Active Retry Receipt

## Scope

This receipt records a development-only fault-injection test run on
2026-09-02. The Remote GPU candidate checked out
`feature/agent-interactive-active-retry` at `1dea0b67` and used Node
`22.12.0`.

The run executed one focused Vitest case with an in-memory Temporal test
server. It did not start Compose, Docker, Kafka, MySQL, a shared Temporal
namespace, or an active Runtime deployment.

## Acceptance

The test models the uncertain boundary after the Core Message command has
committed and before the Runtime receives its response:

1. The first message command records its deterministic invocation ID in a
   controlled side-effect store, then returns gRPC `UNAVAILABLE`.
2. The Temporal Activity fails and retries.
3. The retry calculates the same invocation and Message command ID from the
   durable Task/Run binding, conversation ID, and message content.
4. The controlled command handler observes one committed side effect across
   two command calls, then returns a normal action reference.
5. The durable Tool Invocation receives one completed terminal audit record.

The focused command was:

```bash
cd services/agent-runtime
DIPOLE_AGENT_TEMPORAL_INTEGRATION=true \
  npx vitest run src/temporal/agent-task-workflow.integration.test.ts \
  --testNamePattern="retries an uncertain interactive message response" \
  --reporter=dot
```

It passed as `1 passed / 13 skipped`. The expected first-attempt warning
records `response lost after Message commit`; the final assertion verifies two
command attempts, one unique command identity, one modeled commit, and one
completed Tool Invocation.

The same candidate also passed the focused message projection suite (`5/5`),
`npm run typecheck`, and `npm run build` under Node `22.12.0`.

## Limits And Follow-up

This receipt proves the Runtime retry contract and the deterministic identity
that a real Message Service can use for idempotency. It does not prove a real
Core/Message database commit, transport loss across processes, Worker
replacement, partial-effect rollback, shared tenant behavior, browser HITL,
MCP expansion, capacity, latency, availability, or a success-rate claim.

The clean same-revision loopback Compose approval/deny receipt remains in
[Interactive Active Remote Receipt](AGENT-INTERACTIVE-ACTIVE-REMOTE-RECEIPT.md).
The missing cross-service fault and rollback evidence remains tracked by
`AD-009`.
