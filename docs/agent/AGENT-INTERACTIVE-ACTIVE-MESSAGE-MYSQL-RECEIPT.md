# Interactive Message MySQL Receipt

## Scope

This receipt records a development-only persistence smoke on 2026-09-02. The
Remote GPU candidate used `feature/agent-interactive-transport-retry` at
`1940b563` with Go `1.27`.

The smoke starts one isolated MySQL `8.4` container on a random loopback
port. It runs the repository migration runner, a real SQLC Message repository,
the authenticated Core-to-Message gRPC adapter, and the response-loss proxy.
The container is removed on exit. Docker Compose, Kafka, Temporal, a shared
tenant, and the default active Runtime are not started.

## Acceptance

The test sends one approved-style `system_message` command through the Core
adapter. Message persists the command, then the response-loss proxy returns
gRPC `UNAVAILABLE`. Core queries the Message receipt using the same deterministic
`client_message_id` and returns the recovered message.

The MySQL assertions require exactly:

1. One `messages` row for the recovered message UUID.
2. One `message_metadata` row for that UUID.
3. Two `user_sync_inbox` rows with two distinct recipients: the sender and the
   target of the direct conversation.

The repeatable command is:

```bash
DIPOLE_GO_BIN=/path/to/go-1.26-or-newer/bin/go \
  scripts/smoke-agent-message-command-recovery.sh
```

On the Remote GPU, the command passed with Go `1.27`; the candidate worktree
remained clean and the MySQL container was removed.

## Limits And Follow-up

This receipt proves MySQL-backed Message receipt recovery and the resulting
dual Timeline projection for one direct message. It does not prove the full
interactive Agent authorization lifecycle, a deployed Compose topology,
Kafka, Temporal worker replacement, cross-host network loss, partial-effect
rollback, browser HITL, capacity, latency, availability, or a success-rate
claim.

The Runtime retry and in-memory gRPC transport boundaries remain recorded in
[Interactive Active Retry Receipt](AGENT-INTERACTIVE-ACTIVE-RETRY-RECEIPT.md)
and [Interactive Message Transport Receipt](AGENT-INTERACTIVE-ACTIVE-MESSAGE-TRANSPORT-RECEIPT.md).
The remaining combined reliability gate is tracked by `AD-009`.
