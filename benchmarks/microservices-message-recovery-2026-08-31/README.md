# Microservice Message Recovery Smoke

## Scope

This Remote GPU receipt validates the narrow post-persistence recovery path for the isolated microservice topology. A WebSocket direct message is persisted, the Message Service is restarted, and the client replays the same `client_message_id`.

## Evidence

- Revision: `53a4edf7c169e6bb184160b21b1b0375668eed50`
- Host profile: Remote GPU, isolated Compose project `dipole-message-recovery-53a4edf7`
- Gateway port: `18453`
- Inbox mode: asynchronous inbox projector enabled
- Restart target: `message`
- Final counts: `messages=1`, `outbox_events=1`, target `user_sync_inbox=1`
- Cleanup: candidate containers, volumes, and networks were all absent after the smoke exited.

The machine had active user sessions and existing GPU workloads. This CPU/Docker-only task used a separate Compose project and did not restart Docker or affect the shared Dipole stack.

## Boundary

The receipt proves only one service-restart point after the first durable message write. Kafka consumer interruption, broker failure, in-flight commit recovery, and other service restart targets remain separate fault-matrix work.

See [`receipt.json`](receipt.json) for the machine-readable result and [the image-isolation runbook](../../docs/architecture/MICROSERVICE-IMAGE-ISOLATION.md) for the repeatable command.
