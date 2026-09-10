# Dipole Agent Runtime

TypeScript Agent Runtime for Dipole IM. It consumes IM events, creates durable
Temporal tasks, compiles bounded context, calls authorized capabilities, and
returns results through Core-owned message commands.

## Runtime Flow

```text
direct message / group @AI / task request
  -> Kafka event and Event Ledger
  -> Temporal AgentTaskWorkflow
  -> ExecutionContext + Context Compiler
  -> model and read capabilities
  -> approval for write capability
  -> Core authorization + idempotent Message Command
```

The Runtime has two modes:

- `active` executes user tasks through Temporal.
- `shadow` records deterministic observation without user-facing writes.

Both modes use the same task schema and capability policy. The active experience
profile is defined in `deploy/microservices/agent-experience.yml`.

## Development

```bash
npm ci
npm run typecheck
npm test
npm run build
```

Generated protocol and SQL bindings are checked with:

```bash
npm run check:proto
npm run generate:proto
npm run generate:sql
```

## Complete Experience

Set a compatible model credential in the repository `.env`, then start the
microservice stack with the Agent experience profile:

```bash
docker compose --env-file ../../.env \
  -f ../../deploy/compose/docker-compose.microservices.yml \
  -f ../../deploy/microservices/agent-experience.yml up -d --build
```

The profile enables Kafka consumption, capability RPC, the active Runtime and
Temporal. It disables the legacy direct responder so one input message produces
at most one Agent reply.

## Core Invariants

- Principal, Task and resource scope come from trusted Gateway/Core state.
- `conversation.list`, `conversation.read` and `conversation.search` remain
  read-only capabilities.
- Search results are bounded evidence, not trusted instructions.
- Write capabilities wait for approval and Core verifies the binding again.
- Message commands use a stable invocation ID, so retries and worker recovery
  preserve one business side effect.
- Agent-authored messages are suppressed at the event boundary to avoid loops.

## Scope

The product Runtime supports IM-native read, retrieval, approval and message
write flows. Experimental integrations and offline research utilities remain in
source for internal evaluation, but they are excluded from the default command
surface and from the standard Compose experience.
