# C2 Gateway primary Delivery seam evidence

This archive validates the default-off, direct C++-to-Go primary seam at clean revision `9589bbc63a380767ac932561fecf438281de9800`. It used the isolated Compose project `dipole-c2-primary-9589bbc`, temporary development mTLS certificates, host ports `18088/19095`, and image `sha256:e9cbffa396cfe2ec13da738adf738a4bc974c9c69e03de4ecc1bc65708703724`. The shared `dipole-node1/2/3` stack remained running.

## Result

- A one-shot C++ `deliver_probe` targeted one live `user_id + connection_id`. Gateway returned `ACCEPTED/ENQUEUED` with one accepted connection, and the client received one frame containing the expected request, trace, event, and stable delivery IDs.
- Repeating the identical batch simulated a lost ACK. Gateway returned the same terminal result while the 45-second client capture retained exactly one primary frame.
- A connection still present in the Redis TTL projection after its Hub client had closed returned `ACCEPTED/OFFLINE`. This confirms stale Presence does not broaden delivery to another connection.
- Gateway service readiness and Kafka assignment readiness were both `1`; Kafka group output is retained as context. The probe does not consume Kafka and therefore provides no Kafka offset claim.

Run `python3 benchmarks/c2-primary-delivery-seam-2026-08-28/verify.py` to regenerate `report.json`. The report remains explicit about two later gates: real partial queue saturation through the C++ probe, and consume-to-ACK offset/crash replay in an explicit primary runtime.

All Presence evidence is sanitized and excludes session token IDs, token values, shared secrets, and certificate material. The isolated containers, network, volumes, and temporary certificate directory were removed after capture.
