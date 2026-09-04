# Interactive Active Smoke Receipt: 2026-09-04

## Scope

This development-only receipt records an isolated Remote GPU Compose smoke at
source revision `c9ff05f0733b5bed5909bce9975c2d2f394d8a0c`. The script used a
deterministic, no-network `/send` fixture and a temporary owner-scoped
promotion grant. It writes the JSON receipt only after every assertion and
grant revocation succeeds.

| Check | Result |
| --- | --- |
| Active Kafka group | Runtime passed active profile validation |
| Denied approval | Zero Tool Invocations and zero messages |
| Approved replay after Worker restart | One completed Tool Invocation and one message |
| Message idempotency | One distinct client message ID |
| Sync projection | Two inbox entries |
| Cleanup | Candidate containers were zero; public `dipole-experience` remained at 12 containers |

The retained Remote GPU log SHA-256 is
`3e3ada1b2c9fe8a73d686870c72a99b433661d0c503d528e94fcfab677cbf1d9`.
The receipt file SHA-256 is
`ebda58c9f34ebb8486e290c6d9c55c14f858af387d60eb9b034753b1ccea46f0`.

## Boundary

The receipt intentionally retains no task ID, owner identity, message text,
prompt, model response, token, credential, or public endpoint. It demonstrates
only the isolated, owner-approved active-write recovery path. Browser HITL,
shared-tenant authority, external OAuth/MCP writes, model quality, capacity,
latency, availability, Core/Message replacement, and partial-effect rollback
remain outside this evidence.
