# Agent MCP and Approval Shadow Drill 2026-09-04

## Scope

This receipt records a disposable Remote GPU run from source revision `8c9c0b3f5d9f246043e0b55b51f8858360cb7c46`. It used isolated MySQL 8.4, Kafka 3.9, an in-memory Temporal test server, a Go Core mTLS fixture, and a local MCP fixture.

## Verified Behavior

- Two subscription-scoped ledger events completed with one allowlisted local Tool call and one Artifact projection.
- Restart replay produced no additional Tool call, and expired readiness rejected a further invocation.
- Core authenticated `dipole-agent` over mTLS and rejected invalid identities.
- The approval gate allowed one approved fixture effect while denied, consumed-replay, and failed-operation replay paths created no additional effect.

## Boundary

The receipts set `production_authority=false`. The run did not contact a real external MCP server or write IM messages. It does not prove browser approval UI behavior, shared Core/Kafka/Temporal authority, public DNS/TLS, credential lifecycle, or active write authority. The script removed its disposable containers, volumes, network, certificates, and temporary fixture after completion; post-run inspection confirmed no candidate containers remained and the public `dipole-experience` project stayed at 12 containers.

See [`mcp-receipt.json`](mcp-receipt.json), [`approval-receipt.json`](approval-receipt.json), and the [External MCP runbook](../../docs/agent/agent-external-mcp.md).
