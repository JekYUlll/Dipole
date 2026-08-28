import type { AgentTaskProjectionReconcileReport } from "../reconcile/agent-task-projection-reconciler.js";
import { offlineEvalCategories, parseOfflineEvalReport, type OfflineEvalReport } from "../evals/offline-evaluator.js";
import { z } from "zod";

const hourMs = 60 * 60 * 1000;
export const agentShadowPromotionPolicy = {
  schemaVersion: "dipole.agent.shadow-promotion-policy.v1",
  minimumWindowHours: 24,
  minimumObservations: 24,
  maximumObservationGapMinutes: 90,
  minimumScannedTasks: 100,
  requiredProjectionEvalCases: 6,
  requiredAgentEvals: ["outcome", "trajectory", "permission"] as const
};

export const agentShadowPromotionPolicyV2 = {
  ...agentShadowPromotionPolicy,
  schemaVersion: "dipole.agent.shadow-promotion-policy.v2",
  requiredAgentEvals: offlineEvalCategories
} as const;

export interface AgentShadowPromotionEvidence {
  schemaVersion: "dipole.agent.shadow-promotion-evidence.v1";
  candidateVersion: string;
  windowStartedAt: string;
  windowEndedAt: string;
  observations: Array<{
    candidateVersion: string;
    observedAt: string;
    report: AgentTaskProjectionReconcileReport;
  }>;
  evals: {
    projectionPassed: number;
    projectionTotal: number;
    outcome: boolean;
    trajectory: boolean;
    permission: boolean;
  };
}

export interface AgentShadowPromotionDecision {
  schemaVersion: "dipole.agent.shadow-promotion-decision.v1";
  candidateVersion: string;
  decision: "eligible" | "blocked";
  reasons: string[];
  observedHours: number;
  observations: number;
  scannedTasks: number;
}

export interface AgentShadowPromotionEvidenceV2 {
  schemaVersion: "dipole.agent.shadow-promotion-evidence.v2";
  candidateVersion: string;
  windowStartedAt: string;
  windowEndedAt: string;
  observations: AgentShadowPromotionEvidence["observations"];
  projectionEvals: { passed: number; total: number };
  offlineEvalReport: OfflineEvalReport;
}

export interface AgentShadowPromotionDecisionV2 {
  schemaVersion: "dipole.agent.shadow-promotion-decision.v2";
  candidateVersion: string;
  decision: "eligible" | "blocked";
  reasons: string[];
  observedHours: number;
  observations: number;
  scannedTasks: number;
  offlineEvalSuiteSha256: string;
}

const outcomeSchema = z.object({
  match: z.number().int().nonnegative(), missing: z.number().int().nonnegative(),
  stale: z.number().int().nonnegative(), ahead: z.number().int().nonnegative(),
  conflict: z.number().int().nonnegative(), unavailable: z.number().int().nonnegative()
}).strict();

const promotionEvidenceSchema = z.object({
  schemaVersion: z.literal("dipole.agent.shadow-promotion-evidence.v1"),
  candidateVersion: z.string().trim().min(1),
  windowStartedAt: z.string().datetime(),
  windowEndedAt: z.string().datetime(),
  observations: z.array(z.object({
    candidateVersion: z.string().trim().min(1),
    observedAt: z.string().datetime(),
    report: z.object({
      schemaVersion: z.literal("dipole.agent.projection-reconcile.v1"),
      consistent: z.boolean(), scanned: z.number().int().nonnegative(), outcomes: outcomeSchema,
      examples: z.array(z.unknown())
    }).strict()
  }).strict()),
  evals: z.object({
    projectionPassed: z.number().int().nonnegative(), projectionTotal: z.number().int().nonnegative(),
    outcome: z.boolean(), trajectory: z.boolean(), permission: z.boolean()
  }).strict()
}).strict();

const promotionEvidenceV2Schema = z.object({
  schemaVersion: z.literal("dipole.agent.shadow-promotion-evidence.v2"),
  candidateVersion: z.string().trim().min(1),
  windowStartedAt: z.string().datetime(),
  windowEndedAt: z.string().datetime(),
  observations: promotionEvidenceSchema.shape.observations,
  projectionEvals: z.object({ passed: z.number().int().nonnegative(), total: z.number().int().nonnegative() }).strict(),
  offlineEvalReport: z.unknown()
}).strict();

export function parseAgentShadowPromotionEvidence(value: unknown): AgentShadowPromotionEvidence {
  return promotionEvidenceSchema.parse(value) as AgentShadowPromotionEvidence;
}

export function parseAgentShadowPromotionEvidenceV2(value: unknown): AgentShadowPromotionEvidenceV2 {
  const parsed = promotionEvidenceV2Schema.parse(value);
  return { ...parsed, offlineEvalReport: parseOfflineEvalReport(parsed.offlineEvalReport) } as AgentShadowPromotionEvidenceV2;
}

export function evaluateAgentShadowPromotion(evidence: AgentShadowPromotionEvidence): AgentShadowPromotionDecision {
  if (evidence.schemaVersion !== "dipole.agent.shadow-promotion-evidence.v1" || evidence.candidateVersion.trim() === "") {
    throw new Error("Agent shadow promotion evidence is invalid");
  }
  const startedAt = timestamp(evidence.windowStartedAt);
  const endedAt = timestamp(evidence.windowEndedAt);
  if (endedAt <= startedAt) throw new Error("Agent shadow promotion window is invalid");

  const reasons = new Set<string>();
  const observedHours = (endedAt - startedAt) / hourMs;
  if (observedHours < agentShadowPromotionPolicy.minimumWindowHours) reasons.add("window_too_short");
  if (evidence.observations.length < agentShadowPromotionPolicy.minimumObservations) reasons.add("insufficient_observations");

  let previous = startedAt;
  let scannedTasks = 0;
  for (const observation of evidence.observations) {
    if (observation.candidateVersion !== evidence.candidateVersion) {
      throw new Error("Agent shadow promotion candidate version drifted within the evidence window");
    }
    const observedAt = timestamp(observation.observedAt);
    if (observedAt < startedAt || observedAt > endedAt || observedAt < previous) {
      throw new Error("Agent shadow promotion observations are outside the ordered evidence window");
    }
    if (observedAt-previous > agentShadowPromotionPolicy.maximumObservationGapMinutes * 60 * 1000) reasons.add("observation_gap");
    previous = observedAt;
    validateReport(observation.report);
    scannedTasks += observation.report.scanned;
    if (observation.report.outcomes.missing + observation.report.outcomes.stale + observation.report.outcomes.ahead + observation.report.outcomes.conflict > 0) {
      reasons.add("projection_mismatch");
    }
    if (observation.report.outcomes.unavailable > 0) reasons.add("workflow_unavailable");
  }
  if (endedAt-previous > agentShadowPromotionPolicy.maximumObservationGapMinutes * 60 * 1000) reasons.add("observation_gap");
  if (scannedTasks < agentShadowPromotionPolicy.minimumScannedTasks) reasons.add("insufficient_task_samples");
  if (evidence.evals.projectionPassed !== evidence.evals.projectionTotal || evidence.evals.projectionTotal < agentShadowPromotionPolicy.requiredProjectionEvalCases) reasons.add("projection_eval_failed");
  if (!evidence.evals.outcome) reasons.add("outcome_eval_failed");
  if (!evidence.evals.trajectory) reasons.add("trajectory_eval_failed");
  if (!evidence.evals.permission) reasons.add("permission_eval_failed");

  return {
    schemaVersion: "dipole.agent.shadow-promotion-decision.v1",
    candidateVersion: evidence.candidateVersion,
    decision: reasons.size === 0 ? "eligible" : "blocked",
    reasons: [...reasons].sort(),
    observedHours,
    observations: evidence.observations.length,
    scannedTasks
  };
}

export function evaluateAgentShadowPromotionV2(evidence: AgentShadowPromotionEvidenceV2): AgentShadowPromotionDecisionV2 {
  const report = parseOfflineEvalReport(evidence.offlineEvalReport);
  if (report.candidateVersion !== evidence.candidateVersion) {
    throw new Error("Agent shadow promotion offline evaluation candidate version does not match evidence candidate version");
  }
  const legacyEvidence: AgentShadowPromotionEvidence = {
    schemaVersion: "dipole.agent.shadow-promotion-evidence.v1",
    candidateVersion: evidence.candidateVersion,
    windowStartedAt: evidence.windowStartedAt,
    windowEndedAt: evidence.windowEndedAt,
    observations: evidence.observations,
    evals: {
      projectionPassed: evidence.projectionEvals.passed,
      projectionTotal: evidence.projectionEvals.total,
      outcome: true,
      trajectory: true,
      permission: true
    }
  };
  const legacyDecision = evaluateAgentShadowPromotion(legacyEvidence);
  const reasons = new Set(legacyDecision.reasons);
  if (!report.passed) reasons.add("offline_eval_failed");
  for (const category of offlineEvalCategories) {
    const summary = report.summary.categories[category];
    if (summary.passed !== summary.total) reasons.add(`${category}_eval_failed`);
  }
  return {
    schemaVersion: "dipole.agent.shadow-promotion-decision.v2",
    candidateVersion: evidence.candidateVersion,
    decision: reasons.size === 0 ? "eligible" : "blocked",
    reasons: [...reasons].sort(),
    observedHours: legacyDecision.observedHours,
    observations: legacyDecision.observations,
    scannedTasks: legacyDecision.scannedTasks,
    offlineEvalSuiteSha256: report.suiteSha256
  };
}

function validateReport(report: AgentTaskProjectionReconcileReport): void {
  if (report.schemaVersion !== "dipole.agent.projection-reconcile.v1" || report.scanned < 0) {
    throw new Error("Agent shadow promotion projection report is invalid");
  }
  const total = Object.values(report.outcomes).reduce((sum, count) => sum + count, 0);
  if (total !== report.scanned) throw new Error("Agent shadow promotion report outcome total does not match scanned Tasks");
}

function timestamp(value: string): number {
  const parsed = Date.parse(value);
  if (!Number.isFinite(parsed)) throw new Error("Agent shadow promotion timestamp is invalid");
  return parsed;
}
