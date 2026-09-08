import { describe, expect, it } from "vitest";

import { buildContextAblationBindingPlan } from "./context-ablation-binding-plan.js";

const hash = (value: string) => value.repeat(64).slice(0, 64);
const manifest = { schemaVersion: "dipole.agent.context-ablation-manifest.v1", experimentId: "experiment:one", candidateVersion: "agent@one", routePrices: [{ route: "example/flash", inputMicrousdPerMillionTokens: 1, outputMicrousdPerMillionTokens: 1 }], cases: [
  { caseSha256: hash("a"), requiredOutputIds: ["artifact:summary:v1"], relevantEvidenceIds: ["evidence:one"] }
] };
const plan = () => ({ schemaVersion: "dipole.agent.context-ablation-binding-plan.v1", experimentId: "experiment:one", candidateVersion: "agent@one", bindings: [
  { caseSha256: hash("a"), condition: "baseline", taskId: "task:one", runId: "run:one" },
  { caseSha256: hash("a"), condition: "retrieval", taskId: "task:two", runId: "run:two" },
  { caseSha256: hash("a"), condition: "memory", taskId: "task:three", runId: "run:three" }
] });

describe("Context ablation binding plan", () => {
  it("requires the complete unique three-condition matrix", () => {
    expect(buildContextAblationBindingPlan(manifest, plan()).bindings.map(item => item.condition)).toEqual(["baseline", "memory", "retrieval"]);
  });

  it("fails closed on matrix, candidate, and run identity drift", () => {
    expect(() => buildContextAblationBindingPlan(manifest, { ...plan(), bindings: plan().bindings.slice(1) })).toThrow(/too small|cover exactly/i);
    expect(() => buildContextAblationBindingPlan(manifest, { ...plan(), candidateVersion: "agent@two" })).toThrow(/candidate version/i);
    expect(() => buildContextAblationBindingPlan(manifest, { ...plan(), bindings: [{ ...plan().bindings[0] }, { ...plan().bindings[1], runId: "run:one" }, plan().bindings[2]] })).toThrow(/run IDs/i);
  });
});
