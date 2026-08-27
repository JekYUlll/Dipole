import { describe, expect, it } from "vitest";

import {
  createConservativeRouteEstimator,
  evaluateTokenCalibration,
  parseRouteContextProfiles,
  type TokenCalibrationCase
} from "./token-estimator.js";

const corpus: readonly TokenCalibrationCase[] = [
  { id: "english", category: "english", text: "Summarize the unresolved migration risks and owners.", referenceTokens: 11 },
  { id: "chinese", category: "chinese", text: "整理尚未解决的数据库迁移风险和负责人。", referenceTokens: 18 },
  { id: "code", category: "code", text: "type Task = { id: string; status: 'running' | 'failed' };", referenceTokens: 21 },
  { id: "emoji", category: "emoji", text: "status: ready ✅ blocked ⛔ review 🔍", referenceTokens: 15 },
  {
    id: "tool-schema", category: "tool_schema",
    text: JSON.stringify({ name: "read_conversation", input: { type: "object", properties: { conversationId: { type: "string" }, limit: { type: "integer" } }, required: ["conversationId"] } }),
    referenceTokens: 47
  }
];

describe("route-aware token estimation", () => {
  it("uses the most conservative estimate and smallest context window across fallback routes", () => {
    const estimator = createConservativeRouteEstimator(["gateway/primary", "gateway/fallback"], [
      { route: "gateway/primary", contextWindowTokens: 32_768, utf8BytesPerToken: 4, safetyMarginBps: 1_000 },
      { route: "gateway/fallback", contextWindowTokens: 16_384, utf8BytesPerToken: 3, safetyMarginBps: 2_000 }
    ]);

    expect(estimator.estimate("abcdefghijkl")).toBe(5);
    expect(estimator.contextWindowTokens).toBe(16_384);
    expect(estimator.fallbackRoutes).toEqual([]);
    expect(estimator.id).toMatch(/^route-calibrated-v1:sha256:/);
  });

  it("uses an explicit deterministic fallback for routes without a profile", () => {
    const estimator = createConservativeRouteEstimator(["gateway/primary", "gateway/unprofiled"], [
      { route: "gateway/primary", contextWindowTokens: 32_768, utf8BytesPerToken: 4, safetyMarginBps: 1_000 }
    ]);

    expect(estimator.fallbackRoutes).toEqual(["gateway/unprofiled"]);
    expect(estimator.contextWindowTokens).toBe(8_192);
    expect(estimator.estimate("abcdefgh")).toBe(5);
  });

  it("evaluates a reproducible multilingual and structured calibration corpus", () => {
    const estimator = createConservativeRouteEstimator(["gateway/calibrated"], [
      { route: "gateway/calibrated", contextWindowTokens: 32_768, utf8BytesPerToken: 2, safetyMarginBps: 2_500 }
    ]);

    expect(evaluateTokenCalibration(estimator, corpus)).toMatchObject({
      estimatorId: estimator.id,
      cases: 5,
      underestimates: [],
      categories: ["chinese", "code", "emoji", "english", "tool_schema"]
    });
  });

  it("rejects duplicate, unknown, or malformed route profile declarations", () => {
    expect(() => parseRouteContextProfiles('[{"route":"a","contextWindowTokens":8192,"utf8BytesPerToken":3,"safetyMarginBps":1000},{"route":"a","contextWindowTokens":8192,"utf8BytesPerToken":3,"safetyMarginBps":1000}]')).toThrow(/unique/);
    expect(() => createConservativeRouteEstimator(["a"], [
      { route: "b", contextWindowTokens: 8_192, utf8BytesPerToken: 3, safetyMarginBps: 1_000 }
    ])).toThrow(/unknown route/);
    expect(() => parseRouteContextProfiles("not-json")).toThrow(/JSON/);
  });
});
