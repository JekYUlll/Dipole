import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { runExternalMcpCredentialLifecycleEvidenceCLI } from "./external-mcp-credential-lifecycle-evidence-cli.js";
import { createExternalMcpCredentialLifecycleEvidence } from "./external-mcp-credential-lifecycle-evidence.js";

describe("external MCP credential lifecycle evidence CLI", () => {
  it("prints one valid evidence document and redacts invalid input", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-mcp-credential-check-"));
    const path = join(directory, "evidence.json");
    const evidence = createExternalMcpCredentialLifecycleEvidence(outcome(), {
      now: () => new Date("2026-08-28T10:00:00.000Z"), validityMs: 60_000
    });
    await writeFile(path, JSON.stringify(evidence), "utf8");
    const output: string[] = [];
    await expect(runExternalMcpCredentialLifecycleEvidenceCLI(
      [`--evidence=${path}`], writer(output), writer([]), () => new Date("2026-08-28T10:00:30.000Z")
    )).resolves.toBe(0);
    expect(JSON.parse(output.join(""))).toEqual(evidence);

    await writeFile(path, JSON.stringify({ token: "must-not-leak" }), "utf8");
    const errors: string[] = [];
    await expect(runExternalMcpCredentialLifecycleEvidenceCLI([`--evidence=${path}`], writer([]), writer(errors)))
      .resolves.toBe(1);
    expect(errors.join("")).toBe("external MCP credential lifecycle evidence is invalid\n");
    expect(errors.join("")).not.toContain(path);
    expect(errors.join("")).not.toContain("must-not-leak");
  });

  it("requires exactly one evidence path", async () => {
    const errors: string[] = [];
    await expect(runExternalMcpCredentialLifecycleEvidenceCLI([], writer([]), writer(errors))).resolves.toBe(1);
    expect(errors.join("")).toMatch(/exactly one/);
  });
});

function outcome() {
  return {
    initial_credential_verified: true,
    rotated_credential_verified: true,
    old_version_revoked_before_transport: true,
    restart_recovered: true,
    active_version_revoked_before_transport: true,
    transport_open_count: 3,
    transport_close_count: 3,
    inflight_revocation_authority: false
  } as const;
}

function writer(values: string[]) {
  return { write: (value: string) => { values.push(String(value)); return true; } };
}
