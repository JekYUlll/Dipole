# C2 C++ Primary Runtime Evidence

This archive binds the default-off C++ primary runtime drill to clean revision `703725b0185d32a95b83e9fc4c03c5d2402e826a`. The isolated Compose project used temporary mTLS certificates, dedicated MySQL/Redis/Kafka/MinIO volumes, host port `18088`, and a dedicated `dipole-realtime-primary-evidence-v1` consumer group. Shared `dipole-node1/2/3` remained running.

## Result

- A canonical direct-created event reached an exact live Gateway connection through C++ `DeliverNodeBatch`. `primary-evidence.v1` recorded `ENQUEUED(1)/commit`; the target partition committed to log end with lag zero.
- A non-reading real WebSocket plus 600 KiB C++ probe batches saturated the Gateway queue. Batch 40 returned `PARTIAL/BACKPRESSURED` with depth/capacity `16/16`, retry hint 25 ms and `QUEUE_FULL`.
- A worker using the same primary group and an unavailable gRPC target wrote repeated `deferred/node_transport/retain` evidence for partition 5 offset 1. Before `SIGKILL`, the group remained at current offset 1, log end 2 and lag 1. The normal worker later replayed the same coordinate, received `ENQUEUED(1)`, committed offset 2 and reduced lag to zero.
- The isolated Gateway's existing Go consumer remained active. Each tested event therefore produced one legacy frame without `delivery_id` and one C++ frame with a stable `delivery_id`. This blocks C3 cutover under `AD-041` until delivery authority is mutually exclusive and rollback is explicit.

Run `python3 benchmarks/c2-primary-runtime-2026-08-28/verify.py` to regenerate `report.json`; then run `sha256sum --check benchmarks/c2-primary-runtime-2026-08-28/SHA256SUMS` after the report is stable.

The archive excludes tokens, user profiles, message bodies, certificates and shared secrets. It claims crash replay after deferred evidence; it does not claim a deterministic crash in the narrow terminal-evidence-to-commit interval.
