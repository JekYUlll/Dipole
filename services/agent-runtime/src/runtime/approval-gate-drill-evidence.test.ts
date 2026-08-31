import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";

import {
  approvalGateDrillEvidenceMaximumValidityMs,
  createApprovalGateDrillEvidence,
  parseApprovalGateDrillEvidence
} from "./approval-gate-drill-evidence.js";

const collectedAt = new Date("2026-08-31T14:00:00.000Z");
const outcome = {
  approved_effect_count: 1,
  denied_effect_count: 0,
  consumed_replay_effect_count: 0,
  failed_effect_count: 1,
  failed_replay_effect_count: 0,
  core_rpc_type: "go_internal_grpc_mtls",
  core_rpc_authenticated: true
} as const;

describe("Approval gate drill evidence", () => {
  it("keeps the language-neutral schema aligned with the Runtime receipt", async () => {
    const schema = JSON.parse(await readFile(new URL("../../../../contracts/agent-approval/v1/approval-gate-drill-evidence.schema.json", import.meta.url), "utf8")) as {
      "x-dipole-version": string; additionalProperties: boolean; required: string[];
    };
    expect(schema["x-dipole-version"]).toBe("dipole.agent.approval-gate-drill.v1");
    expect(schema.additionalProperties).toBe(false);
    expect(schema.required).toContain("failed_replay_effect_count");
  });

  it("creates a canonical, low-sensitive expiring receipt", () => {
    const evidence = createApprovalGateDrillEvidence(outcome, { now: () => collectedAt, validityMs: 60_000 });
    expect(evidence).toMatchObject({ ...outcome, outcome: "passed", production_authority: false, collected_at: collectedAt.toISOString() });
    expect(parseApprovalGateDrillEvidence(evidence, { now: () => new Date("2026-08-31T14:00:30.000Z") })).toEqual(evidence);
  });

  it("rejects outcome drift, digest tampering and invalid time windows", () => {
    const evidence = createApprovalGateDrillEvidence(outcome, { now: () => collectedAt, validityMs: 60_000 });
    for (const value of [
      { ...evidence, denied_effect_count: 1 },
      { ...evidence, production_authority: true },
      { ...evidence, content_sha256: "f".repeat(64) }
    ]) {
      expect(() => parseApprovalGateDrillEvidence(value, { now: () => new Date("2026-08-31T14:00:30.000Z") })).toThrow();
    }
    expect(() => createApprovalGateDrillEvidence(outcome, { validityMs: approvalGateDrillEvidenceMaximumValidityMs + 1 })).toThrow();
  });
});
