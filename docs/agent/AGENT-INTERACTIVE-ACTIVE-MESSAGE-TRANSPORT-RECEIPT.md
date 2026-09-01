# Interactive Message Transport Receipt

## Scope

This receipt records a development-only Core-to-Message gRPC fault-injection
test on 2026-09-02. The Remote GPU candidate checked out
`feature/agent-interactive-transport-retry` at `cc025671` and used Go
`1.27`.

The test uses a real Message gRPC server and authenticated Core client over
`bufconn`. The Message application is an in-memory test model. It does not
start Docker, Compose, MySQL, Kafka, Temporal, a shared namespace, or a
default active Runtime.

## Acceptance

The test verifies the receiver-side receipt recovery that closes the boundary
between Core and Message:

1. A trusted Core client sends a `system_message` with a deterministic
   `client_message_id`.
2. The Message server persists one modeled Message, then a proxy returns gRPC
   `UNAVAILABLE` before forwarding the response.
3. `LocalAgentCommandV1` uses the same authenticated Message client to query
   `GetMessageCommandReceipt`.
4. The receipt returns the matching sender, direct-conversation binding,
   content, message type, and `client_message_id`.
5. Core returns the recovered Message and the modeled commit count remains
   exactly one.

The Remote GPU verification command was:

```bash
PATH=/home/admin1/.local/go-1.27.0/bin:$PATH GOTOOLCHAIN=local \
  go test ./internal/services/agent/application ./internal/transport/grpc/message -count=1
```

Both packages passed. The focused transport test is
`TestCoreAgentMessageCommandRecoversCommittedMessageAfterGRPCResponseLoss`.

## Limits And Follow-up

This proves the Core/Message RPC adapter, service authentication, protobuf
binding, receipt query, and recovery behavior in one gRPC transport test. It
does not prove a MySQL commit, external network loss, separate process
replacement, a Temporal retry, partial-effect rollback, shared tenant
behavior, browser HITL, capacity, latency, availability, or a success-rate
claim.

The Runtime-side deterministic Activity retry remains documented in
[Interactive Active Retry Receipt](AGENT-INTERACTIVE-ACTIVE-RETRY-RECEIPT.md).
The clean loopback Compose approval receipt remains documented in
[Interactive Active Remote Receipt](AGENT-INTERACTIVE-ACTIVE-REMOTE-RECEIPT.md).
The full cross-service fault and rollback gate remains tracked by `AD-009`.
