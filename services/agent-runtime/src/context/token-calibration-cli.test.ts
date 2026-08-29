import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { runTokenCalibrationCLI } from "./token-calibration-cli.js";

describe("Context token calibration CLI", () => {
  it("emits an eligible report without provider access", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-context-calibration-"));
    const path = join(directory, "evidence.json");
    await writeFile(path, JSON.stringify(evidence()), "utf8");
    const stdout = collector();
    const stderr = collector();

    await expect(runTokenCalibrationCLI([`--evidence=${path}`], stdout, stderr)).resolves.toBe(0);
    expect(JSON.parse(stdout.value)).toMatchObject({ eligible: true, caseCount: 5 });
    expect(stderr.value).toBe("");
  });

  it("uses exit 2 for valid but underestimating evidence and exit 1 for invalid input", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-context-calibration-"));
    const blockedPath = join(directory, "blocked.json");
    const blocked = evidence();
    blocked.profiles[0]!.utf8BytesPerToken = 16;
    blocked.profiles[0]!.safetyMarginBps = 0;
    await writeFile(blockedPath, JSON.stringify(blocked), "utf8");

    await expect(runTokenCalibrationCLI([`--evidence=${blockedPath}`], collector(), collector())).resolves.toBe(2);
    await expect(runTokenCalibrationCLI([], collector(), collector())).resolves.toBe(1);
    await expect(runTokenCalibrationCLI([`--evidence=${blockedPath}`, "--unexpected=true"], collector(), collector())).resolves.toBe(1);
  });
});

function collector(): { value: string; write(chunk: string | Uint8Array): boolean } {
  return {
    value: "",
    write(chunk) {
      this.value += chunk.toString();
      return true;
    }
  };
}

function evidence() {
  const route = "gateway/calibrated";
  const source = { kind: "provider_tokenizer", provider: "fixture", model: "fixture", revision: "v1" };
  return {
    version: "dipole.agent.context-calibration.evidence.v1",
    candidate: "2e4babfb766aed4c512844653bf76622452db61c",
    capturedAt: "2026-08-27T08:00:00.000Z",
    dataClassification: "synthetic",
    routes: [route],
    profiles: [{ route, contextWindowTokens: 32_768, utf8BytesPerToken: 2, safetyMarginBps: 2_500 }],
    cases: [
      ["english", "english", "Summarize migration risks and owners.", 8],
      ["chinese", "chinese", "整理数据库迁移风险和负责人。", 14],
      ["code", "code", "type Task = { id: string; status: string };", 16],
      ["emoji", "emoji", "ready ✅ blocked ⛔ review 🔍", 12],
      ["tool", "tool_schema", '{"name":"read_conversation","input":{"type":"object"}}', 20]
    ].map(([id, category, text, referenceTokens]) => ({ id, route, category, text, referenceTokens, source }))
  };
}
