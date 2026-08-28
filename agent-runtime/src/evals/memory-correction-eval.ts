import { createHash } from "node:crypto";

import { z } from "zod";

import { parseOfflineEvalSuite, type OfflineEvalSuite } from "./offline-evaluator.js";

const publicIdSchema = z.string().trim().min(1).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]*$/);
const stepSchema = z.string().trim().min(2).max(128).regex(/^[a-z0-9][a-z0-9._:-]*$/);
const candidateVersionSchema = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/);
const metricsSchema = z.object({
  modelCalls: z.number().int().nonnegative(), toolCalls: z.number().int().nonnegative(),
  totalTokens: z.number().int().nonnegative(), totalCostMicrousd: z.number().int().nonnegative(),
  latencyMs: z.number().int().nonnegative().safe()
}).strict().superRefine((metrics, context) => {
  if (metrics.modelCalls !== 0 || metrics.toolCalls !== 0 || metrics.totalTokens !== 0 || metrics.totalCostMicrousd !== 0) {
    context.addIssue({ code: "custom", message: "Memory correction Eval requires zero-model and zero-Tool evidence" });
  }
});

const manifestSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-correction-eval-manifest.v1"),
  candidateVersion: candidateVersionSchema,
  expectedSourceVersion: z.number().int().positive().max(4_294_967_294),
  expectedTrajectory: z.array(stepSchema).min(1).max(32),
  maximumLatencyMs: z.number().int().positive().safe()
}).strict();

const observationSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-correction-eval-observation.v1"),
  memoryRootId: publicIdSchema,
  previous: z.object({
    memoryId: publicIdSchema, memoryVersion: z.number().int().positive().max(4_294_967_294),
    status: z.literal("revoked"), revokedByRole: z.literal("owner")
  }).strict(),
  corrected: z.object({
    memoryId: publicIdSchema, memoryVersion: z.number().int().min(2).max(4_294_967_295), status: z.literal("active"),
    supersedesMemoryId: publicIdSchema, correctedByRole: z.literal("owner"),
    provenanceSourceType: z.literal("owner_correction"), provenanceSourceId: publicIdSchema,
    provenanceSequence: z.string().regex(/^[1-9][0-9]*$/)
  }).strict(),
  trajectory: z.array(stepSchema).min(1).max(32),
  authorizationChecks: z.array(z.object({
    actorRole: z.enum(["owner", "foreign"]), decision: z.enum(["allowed", "denied"])
  }).strict()).max(2).superRefine((checks, context) => {
    if (checks.length !== 2) context.addIssue({ code: "custom", message: "Memory correction permission evidence requires owner and foreign checks" });
  }),
  retrievedMemoryIds: z.array(publicIdSchema).max(16).refine(values => new Set(values).size === values.length, "retrieved Memory identities must be unique"),
  activeRootRecordCount: z.number().int().nonnegative().max(16),
  lineageRecordCount: z.number().int().positive().max(4_294_967_295),
  exactReplayConverged: z.boolean(),
  driftConflict: z.boolean(),
  metrics: metricsSchema
}).strict();

export type MemoryCorrectionEvalManifest = z.infer<typeof manifestSchema>;
export type MemoryCorrectionEvalObservation = z.infer<typeof observationSchema>;

export function parseMemoryCorrectionEvalManifest(value: unknown): MemoryCorrectionEvalManifest {
  return manifestSchema.parse(typeof value === "string" ? JSON.parse(value) as unknown : value);
}

export function parseMemoryCorrectionEvalObservation(value: unknown): MemoryCorrectionEvalObservation {
  return observationSchema.parse(typeof value === "string" ? JSON.parse(value) as unknown : value);
}

export function buildMemoryCorrectionEvalSuite(
  manifest: MemoryCorrectionEvalManifest,
  observation: MemoryCorrectionEvalObservation
): OfflineEvalSuite {
  const expected = manifestSchema.parse(manifest);
  const observed = observationSchema.parse(observation);
  assertCorrectionEvidence(expected, observed);

  const source = digest([observed.memoryRootId, observed.previous.memoryId, observed.corrected.memoryId]).slice(0, 24);
  const resourceId = `memory:${digest([observed.memoryRootId]).slice(0, 32)}`;
  const correctedEvidenceId = memoryEvidenceId(observed.corrected.memoryId);
  const permissionDecisions = [
    { capabilityId: "memory.correct", resourceType: "memory", resourceId, action: "correct", decision: "allowed" as const },
    { capabilityId: "memory.correct", resourceType: "memory", resourceId, action: "correct_foreign", decision: "denied" as const }
  ];

  return parseOfflineEvalSuite({
    schemaVersion: "dipole.agent.offline-eval-suite.v1",
    candidateVersion: expected.candidateVersion,
    cases: [
      {
        id: `outcome.memory_correction.${source}`, category: "outcome",
        expected: {
          requiredOutputIds: ["memory:previous:revoked", "memory:successor:active", "lineage:complete", "replay:exact", "conflict:drift"],
          forbiddenOutputIds: ["lineage:fork", "memory:in_place_mutation"]
        },
        observed: { outputIds: ["memory:previous:revoked", "memory:successor:active", "lineage:complete", "replay:exact", "conflict:drift"] }
      },
      {
        id: `trajectory.memory_correction.${source}`, category: "trajectory",
        expected: { steps: expected.expectedTrajectory, forbiddenSteps: ["memory.in_place_update", "model.call", "tool.call"] },
        observed: { steps: observed.trajectory }
      },
      {
        id: `permission.memory_correction.${source}`, category: "permission",
        expected: { decisions: permissionDecisions }, observed: { decisions: permissionDecisions }
      },
      {
        id: `retrieval.memory_correction.${source}`, category: "retrieval",
        expected: { relevantEvidenceIds: [correctedEvidenceId], minimumRecall: 1, minimumPrecision: 1 },
        observed: { retrievedEvidenceIds: observed.retrievedMemoryIds.map(memoryEvidenceId) }
      },
      {
        id: `cost.memory_correction.${source}`, category: "cost",
        expected: { maximums: { modelCalls: 0, toolCalls: 0, totalTokens: 0, totalCostMicrousd: 0, latencyMs: expected.maximumLatencyMs } },
        observed: observed.metrics
      }
    ]
  });
}

function assertCorrectionEvidence(manifest: MemoryCorrectionEvalManifest, observation: MemoryCorrectionEvalObservation): void {
  if (observation.previous.memoryVersion !== manifest.expectedSourceVersion || observation.corrected.memoryVersion !== manifest.expectedSourceVersion + 1) {
    throw new Error("Memory correction version binding is invalid");
  }
  if (observation.corrected.supersedesMemoryId !== observation.previous.memoryId || observation.corrected.provenanceSourceId !== observation.previous.memoryId) {
    throw new Error("Memory correction predecessor binding is invalid");
  }
  if (observation.corrected.provenanceSequence !== String(observation.corrected.memoryVersion)) {
    throw new Error("Memory correction provenance sequence is invalid");
  }
  if (!equalList(observation.trajectory, manifest.expectedTrajectory)) throw new Error("Memory correction trajectory is invalid");
  if (!observation.exactReplayConverged || !observation.driftConflict) throw new Error("Memory correction replay evidence is invalid");
  if (observation.activeRootRecordCount !== 1 || observation.lineageRecordCount !== observation.corrected.memoryVersion) {
    throw new Error("Memory correction lineage count is invalid");
  }
  const decisions = new Map(observation.authorizationChecks.map(item => [item.actorRole, item.decision]));
  if (decisions.size !== 2 || decisions.get("owner") !== "allowed" || decisions.get("foreign") !== "denied") {
    throw new Error("Memory correction permission evidence is invalid");
  }
  if (observation.retrievedMemoryIds.length !== 1 || observation.retrievedMemoryIds[0] !== observation.corrected.memoryId) {
    throw new Error("Memory correction retrieval evidence is invalid");
  }
  if (observation.metrics.latencyMs > manifest.maximumLatencyMs) throw new Error("Memory correction latency evidence exceeds the maximum");
}

function memoryEvidenceId(memoryId: string): string {
  return `evidence:${digest([memoryId]).slice(0, 32)}`;
}

function digest(values: readonly string[]): string {
  const hash = createHash("sha256");
  for (const value of values) hash.update(`${Buffer.byteLength(value, "utf8")}:${value}|`, "utf8");
  return hash.digest("hex");
}

function equalList(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}
