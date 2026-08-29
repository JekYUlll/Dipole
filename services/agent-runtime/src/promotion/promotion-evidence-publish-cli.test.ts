import { writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it, vi } from "vitest";

import { runPromotionEvidencePublishCLI } from "./promotion-evidence-publish-cli.js";

describe("promotion evidence publication CLI", () => {
  it("publishes one input and emits only the receipt", async () => {
    const path = join(tmpdir(), `dipole-promotion-publish-${process.pid}-${Date.now()}.json`);
    const input = { schemaVersion: "dipole.agent.promotion-evidence-publication.v1" as const, tenantId: "TENANT-A", taskId: "TASK-1", runId: "RUN-1", evidence: { sensitive: "body" } };
    await writeFile(path, JSON.stringify(input), "utf8");
    const output: string[] = [];
    const errors: string[] = [];
    const close = vi.fn();
    const publish = vi.fn(async () => ({
      schemaVersion: "dipole.agent.promotion-evidence-receipt.v1" as const, artifactId: "a".repeat(64), evidenceSHA256: "b".repeat(64),
      evalSuiteSHA256: "e".repeat(64), tenantId: "TENANT-A", taskId: "TASK-1", runId: "RUN-1", runtimeId: "dipole-agent" as const,
      candidateVersion: "candidate/v1", definitionId: "DEF-1", definitionVersion: 1
    }));

    await expect(runPromotionEvidencePublishCLI([`--input=${path}`], writer(output), writer(errors), {
      openPublisher: () => ({ publish, close })
    })).resolves.toBe(0);
    expect(publish).toHaveBeenCalledWith(input);
    expect(close).toHaveBeenCalledOnce();
    expect(output.join("")).not.toContain("sensitive");
    expect(JSON.parse(output.join(""))).toMatchObject({ schemaVersion: "dipole.agent.promotion-evidence-receipt.v1", artifactId: "a".repeat(64) });
    expect(errors).toEqual([]);
  });

  it("fails closed and closes the RPC handle", async () => {
    const path = join(tmpdir(), `dipole-promotion-publish-invalid-${process.pid}-${Date.now()}.json`);
    await writeFile(path, "{}", "utf8");
    const close = vi.fn();
    const errors: string[] = [];
    await expect(runPromotionEvidencePublishCLI([`--input=${path}`], writer([]), writer(errors), {
      openPublisher: () => ({ publish: async () => { throw new Error("blocked evidence"); }, close })
    })).resolves.toBe(1);
    expect(close).toHaveBeenCalledOnce();
    expect(errors.join("")).toContain("failed closed");
  });
});

function writer(values: string[]) {
  return { write: (value: string) => { values.push(String(value)); return true; } };
}
