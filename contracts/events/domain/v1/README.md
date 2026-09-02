# Domain Events v1

These schemas define the remaining Kafka-managed collaboration events used by
Dipole Core and Gateway. They are language-neutral inputs for future
TypeScript Agent and C++ Delivery clients.

- `group-event.schema.json` covers Group lifecycle and membership facts.
- `conversation-direct-read.schema.json` covers direct read receipts.
- `contact-friend-deleted.schema.json` covers bilateral contact removal.
- `session-force-logout.schema.json` covers connection revocation commands.

Producers emit every required field and `version: v1`. Consumers accept a
missing version as legacy v1, ignore additive fields within `v1.x`, and reject
unknown event types. A breaking field or semantic change requires a new major
schema directory and a dual-read rollout.
