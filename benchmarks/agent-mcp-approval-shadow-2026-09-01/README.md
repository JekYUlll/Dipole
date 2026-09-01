# Agent MCP and Approval Shadow Drill

## Scope

This receipt records one disposable Remote GPU run of the Agent external-MCP and approval-gate drills. The candidate source revision was `3c1f3eba87921419ff7186b1ea7ff09d1a7206f9`.

The run started isolated MySQL 8.4, Kafka 3.9, an in-memory Temporal test server, a Go Core mTLS fixture, and a local MCP fixture. The drill removed its containers, volumes, network, certificates, and temporary fixture when it completed.

## Verified behavior

- The MCP path completed two ledger events, one allowlisted local Tool call, and one Artifact projection.
- Replaying the same event after Runtime restart did not create a second Tool call.
- An expired readiness receipt prevented a new Tool call.
- The Core fixture accepted the authenticated `dipole-agent` mTLS identity and rejected invalid identities.
- The approval gate allowed one bound fixture operation exactly once. A deliberately failing operation did not replay after its approval was consumed.

## Boundary

Both receipts declare `production_authority=false`. The MCP endpoint was a local fixture; no shared Core, Kafka, Temporal, external MCP provider, IM message write, public DNS/TLS path, credential lifecycle, approval UI, or service-side commit-uncertainty path was exercised. The approval receipt's zero denied effects is an effect count, not evidence of an approval-deny UI flow.

See [`mcp-receipt.json`](mcp-receipt.json), [`approval-receipt.json`](approval-receipt.json), and the [External MCP runbook](../../docs/agent/agent-external-mcp.md) for the repeatable command and protocol boundary.
