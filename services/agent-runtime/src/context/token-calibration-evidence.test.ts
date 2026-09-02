import { readFile } from "node:fs/promises";

import { describe, expect, it } from "vitest";

import {
  evaluateCalibrationEvidence,
  parseCalibrationEvidenceJSON,
  type TokenCalibrationEvidence
} from "./token-calibration-evidence.js";

describe("Context token calibration evidence", () => {
  it("produces deterministic hash-bound evidence without returning corpus text", () => {
    const evidence = validEvidence();

    const first = evaluateCalibrationEvidence(evidence);
    const second = evaluateCalibrationEvidence(structuredClone(evidence));

    expect(first).toEqual(second);
    expect(first).toMatchObject({
      version: "dipole.agent.context-calibration.report.v1",
      candidate: evidence.candidate,
      eligible: true,
      fallbackRoutes: [],
      caseCount: 5,
      categories: ["chinese", "code", "emoji", "english", "tool_schema"],
      underestimates: []
    });
    expect(first.evidenceSha256).toMatch(/^[a-f0-9]{64}$/);
    expect(first.reportSha256).toMatch(/^[a-f0-9]{64}$/);
    expect(JSON.stringify(first)).not.toContain("migration risks and owners");
    expect(first.measurements[0]).toMatchObject({ textSha256: expect.stringMatching(/^[a-f0-9]{64}$/), utf8Bytes: expect.any(Number) });
  });

  it("blocks a profile that underestimates its own route reference", () => {
    const evidence = validEvidence();
    evidence.profiles[0] = { ...evidence.profiles[0]!, utf8BytesPerToken: 16, safetyMarginBps: 0 };

    const report = evaluateCalibrationEvidence(evidence);

    expect(report.eligible).toBe(false);
    expect(report.underestimates.length).toBeGreaterThan(0);
    expect(report.underestimates[0]).toMatchObject({ route: "gateway/calibrated", id: expect.any(String) });
  });

  it("requires every configured route to cover the fixed corpus categories", () => {
    const evidence = validEvidence();
    evidence.cases.pop();

    expect(() => evaluateCalibrationEvidence(evidence)).toThrow(/all required categories/);
  });

  it("rejects unknown routes, additional fields, and trailing JSON", () => {
    const evidence = validEvidence();
    evidence.cases[0] = { ...evidence.cases[0]!, route: "gateway/unknown" };
    expect(() => evaluateCalibrationEvidence(evidence)).toThrow(/configured route/);
    expect(() => parseCalibrationEvidenceJSON(`${JSON.stringify(validEvidence())}\n{}`)).toThrow(/single JSON value/);
    expect(() => parseCalibrationEvidenceJSON(JSON.stringify({ ...validEvidence(), extra: true }))).toThrow();
  });

  it("reproduces the language-neutral golden evidence and report", async () => {
    const evidence = parseCalibrationEvidenceJSON(await readFile(
      new URL("../../../../contracts/agent-context-calibration/v1/examples/eligible-evidence.json", import.meta.url), "utf8"
    ));
    const report = JSON.parse(await readFile(
      new URL("../../../../contracts/agent-context-calibration/v1/examples/eligible-report.json", import.meta.url), "utf8"
    ));

    expect(evaluateCalibrationEvidence(evidence)).toEqual(report);
  });
});

function validEvidence(): TokenCalibrationEvidence {
  const route = "gateway/calibrated";
  const source = {
    kind: "provider_tokenizer" as const,
    provider: "fixture-provider",
    model: "fixture-model",
    revision: "fixture-tokenizer-v1"
  };
  return {
    version: "dipole.agent.context-calibration.evidence.v1",
    candidate: "2e4babfb766aed4c512844653bf76622452db61c",
    capturedAt: "2026-08-27T08:00:00.000Z",
    dataClassification: "synthetic",
    routes: [route],
    profiles: [{ route, contextWindowTokens: 32_768, utf8BytesPerToken: 2, safetyMarginBps: 2_500 }],
    cases: [
      { id: "english", route, category: "english", text: "Summarize migration risks and owners.", referenceTokens: 8, source },
      { id: "chinese", route, category: "chinese", text: "整理数据库迁移风险和负责人。", referenceTokens: 14, source },
      { id: "code", route, category: "code", text: "type Task = { id: string; status: string };", referenceTokens: 16, source },
      { id: "emoji", route, category: "emoji", text: "ready ✅ blocked ⛔ review 🔍", referenceTokens: 12, source },
      { id: "tool", route, category: "tool_schema", text: '{"name":"read_conversation","input":{"type":"object"}}', referenceTokens: 20, source }
    ]
  };
}
