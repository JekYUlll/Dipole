import { writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { runMemoryPromotionWorkerDrillCLI } from "./memory-promotion-worker-drill-cli.js";

describe("Memory promotion Worker drill CLI", () => {
  it("returns eligible only for a complete evidence record", async () => {
    const path = join(tmpdir(), `dipole-memory-promotion-drill-${process.pid}-${Date.now()}.json`);
    await writeFile(path, JSON.stringify(evidence()), "utf8");
    const output: string[] = [];
    const errors: string[] = [];

    await expect(runMemoryPromotionWorkerDrillCLI([`--evidence=${path}`], sink(output), sink(errors))).resolves.toBe(0);
    expect(JSON.parse(output.join(""))).toMatchObject({ decision: "eligible", reasons: [] });
    await expect(runMemoryPromotionWorkerDrillCLI([], sink(output), sink(errors))).resolves.toBe(1);
    expect(errors.join(" ")).toContain("exactly one --evidence");
  });
});

function evidence(): object {
  return {
    schemaVersion: "dipole.agent.memory-promotion-worker-drill-evidence.v1", sharedEnvironment: true,
    runtimeRevision: "a".repeat(40), candidateVersion: "agent-runtime@receipt-worker-v1",
    releaseManifestSha256: "b".repeat(64), configurationSha256: "c".repeat(64), promotionEvidenceReceiptSha256: "d".repeat(64),
    grantId: "PROMOTION-GRANT-1", temporalTaskQueue: "dipole-agent-memory-promotion-v1", temporalActivityMode: "promotion_active",
    coreReceiptCommitEnabled: true, capabilityRpcMTLS: true, operatorAuthority: "operator_approved",
    firstCommit: { receiptId: `MEM-PROMOTE-${"e".repeat(64)}`, receiptSha256: "e".repeat(64), memoryId: "MEM-1", outcome: "committed" },
    retry: { receiptId: `MEM-PROMOTE-${"e".repeat(64)}`, receiptSha256: "e".repeat(64), memoryId: "MEM-1", outcome: "replayed" },
    revokedGrant: "denied", rollback: "rolled_back", observedAt: "2026-08-30T12:00:00.000Z"
  };
}

function sink(values: string[]): { write(value: string): boolean } {
  return { write: (value) => { values.push(String(value)); return true; } };
}
