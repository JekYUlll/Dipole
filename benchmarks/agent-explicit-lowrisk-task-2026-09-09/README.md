# Explicit Low-Risk Agent Task Receipt

## Scope

Development-only verification of the public `dipole-experience` stack on
Remote GPU. The check used a newly registered test user and the authenticated
Gateway task-control API. It did not enable persistent Memory, subscriptions,
external MCP, OAuth callback handling, or a new deployment profile.

## Result

- `POST /api/v1/agent/tasks` returned `accepted`.
- The persisted task used `lowrisk-assistant:v1`.
- Temporal completed the task and the owner Timeline contained five events.
- The task produced exactly one direct Agent reply.
- Agent, Core, Gateway, Message, Sync, Temporal, Redis, Kafka, MinIO, and
  MySQL remained healthy after the check; the public tunnel was not restarted.

## Boundary

This receipt proves the ordinary interactive-task fallback is usable without
an owner Definition. Subscription triggers and any expanded capability remain
subject to their reviewed promotion-grant requirements. It is not evidence for
default persistent Memory, model quality, or production authority.
