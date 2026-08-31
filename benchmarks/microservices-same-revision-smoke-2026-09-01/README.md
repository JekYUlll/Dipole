# Same-Revision Microservice Smoke

## Scope

This Remote GPU receipt validates the isolated Core, Gateway, Message, Sync, Kafka, Redis, MySQL, MinIO, and Agent Runtime topology from one clean source revision. A direct WebSocket message is persisted, Core is restarted, and the receipt verifies exact message, Outbox, and Inbox counts.

## Evidence

- Revision: `676a6d93cb50ad48a3b254999be14af783456f3e`
- Host profile: Remote GPU isolated Compose project `dipole-smoke-676a6d93`
- Gateway port: `18092`
- Inbox mode: atomic Message-owned write path
- Restart target: Core
- Final counts: `messages=1`, `outbox_events=1`, target `user_sync_inbox=1`
- Agent image: built from the same clean source revision as the Go service images
- Cleanup: the smoke script removed its Compose containers, volumes, and network after receipt creation.

## Boundary

The result verifies one direct-message recovery path with atomic Inbox ownership. It does not enable Agent active authority, exercise interactive Agent Task creation, establish a Cassandra primary read window, or replace the A6 browser observation acceptance criteria.

See [`receipt.json`](receipt.json) and [the image-isolation runbook](../../docs/architecture/MICROSERVICE-IMAGE-ISOLATION.md) for the repeatable command.
