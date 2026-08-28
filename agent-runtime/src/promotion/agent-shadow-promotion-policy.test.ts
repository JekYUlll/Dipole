import { describe, expect, it } from "vitest";
import { readFile } from "node:fs/promises";

import { evaluateOfflineEvalSuite, parseOfflineEvalSuite } from "../evals/offline-evaluator.js";
import {
  agentShadowPromotionPolicy, agentShadowPromotionPolicyV2, evaluateAgentShadowPromotion,
  evaluateAgentShadowPromotionV2, parseAgentShadowPromotionEvidenceV2, type AgentShadowPromotionEvidence,
  type AgentShadowPromotionEvidenceV2
} from "./agent-shadow-promotion-policy.js";

describe("Agent shadow promotion policy", () => {
  it("matches the versioned language-neutral policy", async () => {
    const path = new URL("../../../contracts/agent-promotion/v1/policy.json", import.meta.url);
    const contract = JSON.parse(await readFile(path, "utf8"));
    expect(contract).toEqual({
      schema_version: agentShadowPromotionPolicy.schemaVersion,
      minimum_window_hours: agentShadowPromotionPolicy.minimumWindowHours,
      minimum_observations: agentShadowPromotionPolicy.minimumObservations,
      maximum_observation_gap_minutes: agentShadowPromotionPolicy.maximumObservationGapMinutes,
      minimum_scanned_tasks: agentShadowPromotionPolicy.minimumScannedTasks,
      required_projection_eval_cases: agentShadowPromotionPolicy.requiredProjectionEvalCases,
      required_agent_evals: [...agentShadowPromotionPolicy.requiredAgentEvals]
    });
  });

  it("matches promotion v2 and requires a candidate-bound five-category report", async () => {
    const policyPath = new URL("../../../contracts/agent-promotion/v2/policy.json", import.meta.url);
    const suitePath = new URL("../../../contracts/agent-evals/v1/offline-suite.json", import.meta.url);
    const policy = JSON.parse(await readFile(policyPath, "utf8"));
    const suite = parseOfflineEvalSuite(await readFile(suitePath, "utf8"));
    suite.candidateVersion = "agent-runtime@abc1234";
    const evidence = cleanEvidenceV2(evaluateOfflineEvalSuite(suite));

    expect(policy).toEqual({
      schema_version: agentShadowPromotionPolicyV2.schemaVersion,
      minimum_window_hours: agentShadowPromotionPolicyV2.minimumWindowHours,
      minimum_observations: agentShadowPromotionPolicyV2.minimumObservations,
      maximum_observation_gap_minutes: agentShadowPromotionPolicyV2.maximumObservationGapMinutes,
      minimum_scanned_tasks: agentShadowPromotionPolicyV2.minimumScannedTasks,
      required_projection_eval_cases: agentShadowPromotionPolicyV2.requiredProjectionEvalCases,
      required_agent_evals: [...agentShadowPromotionPolicyV2.requiredAgentEvals]
    });
    expect(evaluateAgentShadowPromotionV2(parseAgentShadowPromotionEvidenceV2(evidence))).toMatchObject({
      schemaVersion: "dipole.agent.shadow-promotion-decision.v2",
      decision: "eligible",
      offlineEvalSuiteSha256: evidence.offlineEvalReport.suiteSha256
    });

    evidence.offlineEvalReport.candidateVersion = "agent-runtime@other";
    expect(() => evaluateAgentShadowPromotionV2(evidence)).toThrow(/candidate version/);
  });

  it("blocks promotion v2 when any offline category fails", async () => {
    const suitePath = new URL("../../../contracts/agent-evals/v1/offline-suite.json", import.meta.url);
    const suite = parseOfflineEvalSuite(await readFile(suitePath, "utf8"));
    suite.candidateVersion = "agent-runtime@abc1234";
    const cost = suite.cases.find(item => item.category === "cost");
    if (cost === undefined) throw new Error("cost fixture missing");
    cost.observed.totalTokens = 5000;

    const decision = evaluateAgentShadowPromotionV2(cleanEvidenceV2(evaluateOfflineEvalSuite(suite)));

    expect(decision.decision).toBe("blocked");
    expect(decision.reasons).toEqual(["cost_eval_failed", "offline_eval_failed"]);
  });
  it("requires a continuous 24-hour clean window and complete Agent Evals", () => {
    const evidence = cleanEvidence();
    expect(evaluateAgentShadowPromotion(evidence)).toEqual({
      schemaVersion: "dipole.agent.shadow-promotion-decision.v1",
      candidateVersion: "agent-runtime@abc1234",
      decision: "eligible",
      reasons: [],
      observedHours: 24,
      observations: 25,
      scannedTasks: 125
    });
  });

  it("blocks insufficient, discontinuous, dirty, unavailable, and incomplete evidence", () => {
    const evidence = cleanEvidence();
    evidence.windowEndedAt = "2026-08-27T23:00:00.000Z";
    evidence.observations.pop();
    evidence.observations.splice(10, 2);
    evidence.observations[0]!.report.outcomes.match -= 1;
    evidence.observations[0]!.report.outcomes.stale = 1;
    evidence.observations[1]!.report.outcomes.match -= 1;
    evidence.observations[1]!.report.outcomes.unavailable = 1;
    evidence.evals.permission = false;

    const decision = evaluateAgentShadowPromotion(evidence);

    expect(decision.decision).toBe("blocked");
    expect(decision.reasons).toEqual(expect.arrayContaining([
      "window_too_short", "observation_gap", "projection_mismatch", "workflow_unavailable", "permission_eval_failed"
    ]));
  });

  it("rejects evidence mixed across candidate versions or malformed report totals", () => {
    const mixed = cleanEvidence();
    mixed.observations[3]!.candidateVersion = "agent-runtime@other";
    expect(() => evaluateAgentShadowPromotion(mixed)).toThrow(/candidate version/);

    const malformed = cleanEvidence();
    malformed.observations[0]!.report.scanned = 4;
    expect(() => evaluateAgentShadowPromotion(malformed)).toThrow(/outcome total/);
  });
});

function cleanEvidence(): AgentShadowPromotionEvidence {
  const started = Date.parse("2026-08-27T00:00:00.000Z");
  return {
    schemaVersion: "dipole.agent.shadow-promotion-evidence.v1",
    candidateVersion: "agent-runtime@abc1234",
    windowStartedAt: new Date(started).toISOString(),
    windowEndedAt: new Date(started + 24 * 60 * 60 * 1000).toISOString(),
    observations: Array.from({ length: 25 }, (_, index) => ({
      candidateVersion: "agent-runtime@abc1234",
      observedAt: new Date(started + index * 60 * 60 * 1000).toISOString(),
      report: {
        schemaVersion: "dipole.agent.projection-reconcile.v1" as const,
        consistent: true,
        scanned: 5,
        outcomes: { match: 5, missing: 0, stale: 0, ahead: 0, conflict: 0, unavailable: 0 },
        examples: []
      }
    })),
    evals: { projectionPassed: 6, projectionTotal: 6, outcome: true, trajectory: true, permission: true }
  };
}

function cleanEvidenceV2(offlineEvalReport: AgentShadowPromotionEvidenceV2["offlineEvalReport"]): AgentShadowPromotionEvidenceV2 {
  const legacy = cleanEvidence();
  return {
    schemaVersion: "dipole.agent.shadow-promotion-evidence.v2",
    candidateVersion: legacy.candidateVersion,
    windowStartedAt: legacy.windowStartedAt,
    windowEndedAt: legacy.windowEndedAt,
    observations: legacy.observations,
    projectionEvals: { passed: 6, total: 6 },
    offlineEvalReport
  };
}
