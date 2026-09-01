# Agent MCP and Approval Shadow Drill v2

## Scope

This receipt records a disposable Remote GPU run from source revision `f0dcf98a0b366031f7097cfd331318d39a9cf7a6`. It ran isolated MySQL 8.4, Kafka 3.9, an in-memory Temporal test server, a Go Core mTLS fixture, and a local MCP fixture.

## Verified behavior

- The MCP path completed two ledger events, one allowlisted local Tool call, and one Artifact projection.
- Runtime restart suppressed a duplicate event, and expired readiness denied a further Tool call.
- The Core fixture authenticated `dipole-agent` over mTLS and rejected invalid identities.
- The approval gate executed one approved fixture operation.
- A denied approval, a consumed approval replay, and a replay after an operation failure were each rejected before an additional effect.

## Boundary

The receipts set `production_authority=false`. The run used local fixtures and wrote no IM message. It does not cover a browser approval UI, shared Core/Kafka/Temporal, a real external MCP server, public DNS/TLS, credential lifecycle, or service-side commit uncertainty. The script removed its dedicated containers, volumes, network, certificates, and temporary fixture after the run.

See [`mcp-receipt.json`](mcp-receipt.json), [`approval-receipt.json`](approval-receipt.json), and the [External MCP runbook](../../docs/agent/agent-external-mcp.md).
