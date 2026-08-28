import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { runMemoryCorrectionEvalCLI } from "./memory-correction-eval-cli.js";

describe("Memory correction Eval CLI", () => {
  it("emits only the standard low-sensitive five-category report", async () => {
    const root = new URL("../../../contracts/agent-evals/v1/", import.meta.url);
    const output: string[] = [];
    const errors: string[] = [];
    const code = await runMemoryCorrectionEvalCLI([
      `--manifest=${new URL("memory-correction-manifest.example.json", root).pathname}`,
      `--observation=${new URL("memory-correction-observation.example.json", root).pathname}`
    ], { write: value => output.push(value) }, { write: value => errors.push(value) });

    expect(code).toBe(0);
    expect(errors).toEqual([]);
    expect(JSON.parse(output.join(""))).toMatchObject({ passed: true, summary: { total: 5, passed: 5 } });
    expect(output.join("")).not.toContain("MEM-ROOT-PRIVATE");
    expect(output.join("")).not.toContain("MEM-SOURCE");
  });

  it("fails closed without echoing invalid evidence", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-memory-eval-"));
    const manifestPath = join(directory, "manifest.json");
    const observationPath = join(directory, "observation.json");
    await writeFile(manifestPath, JSON.stringify(manifestFixture()), "utf8");
    await writeFile(observationPath, JSON.stringify({ secretContent: "Owner is Bob" }), "utf8");
    const output: string[] = [];
    const errors: string[] = [];

    const code = await runMemoryCorrectionEvalCLI([
      `--manifest=${manifestPath}`, `--observation=${observationPath}`
    ], { write: value => output.push(value) }, { write: value => errors.push(value) });

    expect(code).toBe(1);
    expect(output).toEqual([]);
    expect(errors.join("")).toBe("Memory correction Eval failed closed\n");
    expect(errors.join("")).not.toContain("Owner is Bob");
  });

  it("rejects incomplete and oversized inputs", async () => {
    const errors: string[] = [];
    expect(await runMemoryCorrectionEvalCLI([], { write: () => undefined }, { write: value => errors.push(value) })).toBe(1);
    expect(errors.join("")).toMatch(/requires exactly/iu);

    const directory = await mkdtemp(join(tmpdir(), "dipole-memory-eval-large-"));
    const manifestPath = join(directory, "manifest.json");
    const observationPath = join(directory, "observation.json");
    await writeFile(manifestPath, `${" ".repeat(65_536)}{}`, "utf8");
    await writeFile(observationPath, "{}", "utf8");
    const oversizedErrors: string[] = [];
    expect(await runMemoryCorrectionEvalCLI([
      `--manifest=${manifestPath}`, `--observation=${observationPath}`
    ], { write: () => undefined }, { write: value => oversizedErrors.push(value) })).toBe(1);
    expect(oversizedErrors.join("")).toBe("Memory correction Eval failed closed\n");
  });
});

function manifestFixture() {
  return {
    schemaVersion: "dipole.agent.memory-correction-eval-manifest.v1",
    candidateVersion: "agent-memory@correction-v1",
    expectedSourceVersion: 1,
    expectedTrajectory: ["memory.owner.get"],
    maximumLatencyMs: 500
  };
}
