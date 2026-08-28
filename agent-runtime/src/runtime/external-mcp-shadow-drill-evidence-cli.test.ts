import { mkdir, mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { runExternalMcpShadowDrillEvidenceCLI } from "./external-mcp-shadow-drill-evidence-cli.js";
import { createExternalMcpShadowDrillEvidence } from "./external-mcp-shadow-drill-evidence.js";

describe("external MCP Shadow drill evidence CLI", () => {
  it("prints one valid low-sensitive evidence document", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-mcp-drill-check-"));
    await mkdir(directory, { recursive: true });
    const path = join(directory, "evidence.json");
    const evidence = createExternalMcpShadowDrillEvidence(outcome(), {
      now: () => new Date("2026-08-28T10:00:00.000Z"), validityMs: 60_000
    });
    await writeFile(path, JSON.stringify(evidence), "utf8");
    const output: string[] = [];
    const errors: string[] = [];
    await expect(runExternalMcpShadowDrillEvidenceCLI(
      [`--evidence=${path}`], writer(output), writer(errors), () => new Date("2026-08-28T10:00:30.000Z")
    )).resolves.toBe(0);
    expect(JSON.parse(output.join(""))).toEqual(evidence);
    expect(errors).toEqual([]);
  });

  it("fails closed without echoing path or invalid content", async () => {
    const path = join(tmpdir(), `dipole-mcp-drill-invalid-${process.pid}.json`);
    await writeFile(path, JSON.stringify({ token: "must-not-leak" }), "utf8");
    const errors: string[] = [];
    await expect(runExternalMcpShadowDrillEvidenceCLI(
      [`--evidence=${path}`], writer([]), writer(errors)
    )).resolves.toBe(1);
    expect(errors.join("")).toBe("external MCP Shadow drill evidence is invalid\n");
    expect(errors.join("")).not.toContain(path);
    expect(errors.join("")).not.toContain("must-not-leak");
  });

  it("requires exactly one evidence path", async () => {
    const errors: string[] = [];
    await expect(runExternalMcpShadowDrillEvidenceCLI([], writer([]), writer(errors))).resolves.toBe(1);
    expect(errors.join("")).toMatch(/exactly one/);
  });
});

function outcome() {
  return {
    event_count: 2,
    ledger_completed_event_count: 2,
    tool_call_count: 1,
    artifact_count: 1,
    restart_duplicate_suppressed: true,
    expired_readiness_denied: true
  } as const;
}

function writer(values: string[]) {
  return { write: (value: string) => { values.push(String(value)); return true; } };
}
