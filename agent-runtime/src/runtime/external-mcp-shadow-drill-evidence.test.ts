import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";

import {
  createExternalMcpShadowDrillEvidence,
  externalMcpShadowDrillEvidenceMaximumValidityMs,
  externalMcpShadowDrillEvidenceSchemaVersion,
  parseExternalMcpShadowDrillEvidence
} from "./external-mcp-shadow-drill-evidence.js";

const collectedAt = new Date("2026-08-28T10:00:00.000Z");
const outcome = {
  event_count: 2,
  ledger_completed_event_count: 2,
  tool_call_count: 1,
  artifact_count: 1,
  restart_duplicate_suppressed: true,
  expired_readiness_denied: true
} as const;

describe("external MCP Shadow drill evidence", () => {
  it("keeps the language-neutral schema aligned with the strict Runtime contract", async () => {
    const path = new URL("../../../contracts/agent-external-mcp/v1/shadow-drill-evidence.schema.json", import.meta.url);
    const schema = JSON.parse(await readFile(path, "utf8")) as {
      $id: string;
      "x-dipole-version": string;
      required: string[];
      additionalProperties: boolean;
      properties: Record<string, { const?: unknown }>;
    };
    expect(schema.$id).toMatch(/agent-external-mcp\/v1\/shadow-drill-evidence\.schema\.json$/);
    expect(schema["x-dipole-version"]).toBe(externalMcpShadowDrillEvidenceSchemaVersion);
    expect(schema.additionalProperties).toBe(false);
    expect(schema.required.sort()).toEqual([
      "schema_version", "outcome", "isolation", "collected_at", "expires_at", "event_count",
      "ledger_completed_event_count", "tool_call_count", "artifact_count", "restart_duplicate_suppressed",
      "expired_readiness_denied", "production_authority", "content_sha256"
    ].sort());
    expect(schema.properties.production_authority?.const).toBe(false);
  });

  it("creates canonical expiring evidence and verifies its content digest", () => {
    const evidence = createExternalMcpShadowDrillEvidence(outcome, {
      now: () => collectedAt,
      validityMs: 60_000
    });
    expect(evidence).toMatchObject({
      ...outcome,
      schema_version: externalMcpShadowDrillEvidenceSchemaVersion,
      outcome: "passed",
      collected_at: collectedAt.toISOString(),
      expires_at: "2026-08-28T10:01:00.000Z",
      production_authority: false
    });
    expect(evidence.content_sha256).toMatch(/^[a-f0-9]{64}$/);
    expect(parseExternalMcpShadowDrillEvidence(evidence, {
      now: () => new Date("2026-08-28T10:00:30.000Z")
    })).toEqual(evidence);
  });

  it("rejects drift, extra fields, tampering, future evidence and stale evidence", () => {
    const evidence = createExternalMcpShadowDrillEvidence(outcome, {
      now: () => collectedAt,
      validityMs: 60_000
    });
    const cases: Array<[unknown, Date]> = [
      [{ ...evidence, tool_call_count: 2 }, new Date("2026-08-28T10:00:30.000Z")],
      [{ ...evidence, extra: true }, new Date("2026-08-28T10:00:30.000Z")],
      [{ ...evidence, restart_duplicate_suppressed: false }, new Date("2026-08-28T10:00:30.000Z")],
      [{ ...evidence, content_sha256: "f".repeat(64) }, new Date("2026-08-28T10:00:30.000Z")],
      [evidence, new Date("2026-08-28T09:59:59.999Z")],
      [evidence, new Date("2026-08-28T10:01:00.000Z")]
    ];
    for (const [candidate, now] of cases) {
      expect(() => parseExternalMcpShadowDrillEvidence(candidate, { now: () => now })).toThrow();
    }
  });

  it("bounds creation validity to one day", () => {
    expect(() => createExternalMcpShadowDrillEvidence(outcome, { validityMs: 0 })).toThrow();
    expect(() => createExternalMcpShadowDrillEvidence(outcome, {
      validityMs: externalMcpShadowDrillEvidenceMaximumValidityMs + 1
    })).toThrow();
    expect(() => createExternalMcpShadowDrillEvidence({
      ...outcome,
      collected_at: "2027-01-01T00:00:00.000Z"
    } as typeof outcome)).toThrow();
  });
});
