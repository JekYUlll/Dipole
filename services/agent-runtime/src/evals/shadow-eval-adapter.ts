import { createHash } from "node:crypto";

import { z } from "zod";

import { parseOfflineEvalSuite, type OfflineEvalSuite } from "./offline-evaluator.js";

const identifierSchema = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:-]*$/);
// Runtime policy scopes use "*" for a bounded resource class wildcard.
// Eval manifests must preserve that scope instead of relabeling it as a concrete ID.
const resourceIdSchema = z.union([z.literal("*"), identifierSchema]);
const identifierListSchema = z.array(identifierSchema).max(256).refine(values => new Set(values).size === values.length, "identifiers must be unique");
const decisionSchema = z.enum(["allowed", "denied"]);
const metricsSchema = z.object({
  modelCalls: z.number().int().nonnegative(), toolCalls: z.number().int().nonnegative(),
  totalTokens: z.number().int().nonnegative(), totalCostMicrousd: z.number().int().nonnegative(),
  latencyMs: z.number().int().nonnegative()
}).strict();

const shadowEvalManifestSchema = z.object({
  schemaVersion: z.literal("dipole.agent.shadow-eval-manifest.v1"),
  candidateVersion: z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/),
  taskId: z.string().trim().min(1).max(64),
  runId: z.string().trim().min(1).max(64),
  labels: z.object({
    outcome: z.object({ requiredOutputIds: identifierListSchema.min(1), forbiddenOutputIds: identifierListSchema }).strict(),
    trajectory: z.object({ steps: z.array(identifierSchema).max(256), forbiddenSteps: identifierListSchema }).strict(),
    permission: z.array(z.object({
      stepNo: z.number().int().positive(), capabilityId: identifierSchema, resourceType: identifierSchema,
      resourceId: resourceIdSchema, action: identifierSchema, decision: decisionSchema
    }).strict()).max(256).refine(items => new Set(items.map(item => item.stepNo)).size === items.length, "permission step numbers must be unique"),
    retrieval: z.object({
      relevantEvidenceIds: identifierListSchema.min(1), minimumRecall: z.number().min(0).max(1), minimumPrecision: z.number().min(0).max(1)
    }).strict(),
    cost: z.object({
      maximums: metricsSchema,
      routePrices: z.array(z.object({
        route: z.string().trim().min(1).max(255),
        inputMicrousdPerMillionTokens: z.number().int().nonnegative().safe(),
        outputMicrousdPerMillionTokens: z.number().int().nonnegative().safe()
      }).strict()).min(1).max(64).refine(items => new Set(items.map(item => item.route)).size === items.length, "model routes must be unique")
    }).strict()
  }).strict()
}).strict();

export type ShadowEvalManifest = z.infer<typeof shadowEvalManifestSchema>;

export interface ShadowEvalObservation {
  readonly taskId: string;
  readonly taskStatus: string;
  // Shadow runs preserve the policy Task lifecycle; the durable Workflow is
  // the terminal task execution record for that mode.
  readonly workflowStatus?: string | null;
  readonly runId: string;
  readonly runStatus: string;
  readonly traceId: string;
  readonly contextManifest: {
    readonly selected: readonly { readonly id: string; readonly provenance: { readonly sourceType: string; readonly sourceId: string } }[];
    readonly omitted: readonly unknown[];
  };
  readonly steps: readonly {
    readonly stepNo: number; readonly capabilityId: string; readonly status: string;
    readonly attemptCount: number; readonly latencyMs: number | null;
    readonly authorization: { readonly resourceType: string; readonly resourceId: string; readonly action: string; readonly decision: "allowed" } | null;
  }[];
  readonly artifacts: readonly { readonly artifactType: string; readonly version: number }[];
  readonly modelCalls: readonly {
    readonly route: string; readonly status: string; readonly inputTokens: number | null;
    readonly outputTokens: number | null; readonly latencyMs: number | null;
  }[];
  readonly toolCalls: readonly { readonly status: string; readonly latencyMs: number | null }[];
}

export function parseShadowEvalManifest(value: unknown): ShadowEvalManifest {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  return shadowEvalManifestSchema.parse(decoded);
}

export function buildShadowEvalSuite(manifest: ShadowEvalManifest, observation: ShadowEvalObservation): OfflineEvalSuite {
  const labels = shadowEvalManifestSchema.parse(manifest).labels;
  assertBinding(manifest, observation);
  assertTerminal(observation);
  requiredTraceId(observation.traceId);

  const artifactIds = observation.artifacts.map(artifactId).sort();
  const outputIds = [...artifactIds, `run:${observation.runStatus}`, `task:${evaluationTaskStatus(observation)}`].sort();
  const steps = [
    ...(observation.contextManifest.selected.length > 0 ? ["context.compile"] : []),
    ...observation.steps.map(item => `capability:${item.capabilityId}:${item.status}`),
    ...artifactIds
  ];
  const decisions = labels.permission.map(label => {
    const observed = observation.steps.find(step => step.stepNo === label.stepNo);
    if (observed === undefined || observed.capabilityId !== label.capabilityId || observed.authorization === null) {
      throw new Error(`Shadow evaluation capability binding is invalid at Step ${label.stepNo}`);
    }
    if (observed.authorization.resourceType !== label.resourceType || observed.authorization.resourceId !== label.resourceId || observed.authorization.action !== label.action || observed.authorization.decision !== label.decision) throw new Error(`Shadow evaluation authorization binding is invalid at Step ${label.stepNo}`);
    return {
      capabilityId: observed.capabilityId, ...observed.authorization
    };
  });
  const retrievedEvidenceIds = [...new Set(observation.contextManifest.selected.map(item => evidenceId(item.provenance)))].sort();
  const costs = costMetrics(manifest, observation);
  const source = sourceBinding(manifest.taskId, manifest.runId);

  return parseOfflineEvalSuite({
    schemaVersion: "dipole.agent.offline-eval-suite.v1",
    candidateVersion: manifest.candidateVersion,
    cases: [{ id: `outcome.shadow.${source}`, category: "outcome", expected: labels.outcome, observed: { outputIds } },
      { id: `trajectory.shadow.${source}`, category: "trajectory", expected: labels.trajectory, observed: { steps } },
      { id: `permission.shadow.${source}`, category: "permission", expected: { decisions: labels.permission.map(({ stepNo: _stepNo, ...item }) => item) }, observed: { decisions } },
      { id: `retrieval.shadow.${source}`, category: "retrieval", expected: labels.retrieval, observed: { retrievedEvidenceIds } },
      { id: `cost.shadow.${source}`, category: "cost", expected: { maximums: labels.cost.maximums }, observed: costs }]
  });
}

function assertBinding(manifest: ShadowEvalManifest, observation: ShadowEvalObservation): void {
  if (manifest.taskId !== observation.taskId || manifest.runId !== observation.runId) {
    throw new Error("Shadow evaluation Task binding does not match the persisted observation");
  }
}

function assertTerminal(observation: ShadowEvalObservation): void {
  if (observation.steps.length > 256 || observation.artifacts.length > 256 || observation.toolCalls.length > 256 || observation.modelCalls.length > 64) {
    throw new Error("Shadow evaluation observation exceeds its bounded collection limits");
  }
  if (!isTerminalStatus(evaluationTaskStatus(observation)) || !isTerminalStatus(observation.runStatus)) {
    throw new Error("Shadow evaluation Task execution and Run must be terminal");
  }
  const invalidStep = observation.steps.find(item => !["completed", "failed", "denied"].includes(item.status));
  if (invalidStep !== undefined) throw new Error(`Shadow evaluation Step ${invalidStep.stepNo} is not terminal`);
  const incompleteStep = observation.steps.find(item => item.latencyMs === null);
  if (incompleteStep !== undefined) throw new Error(`Shadow evaluation Step ${incompleteStep.stepNo} has incomplete latency`);
  const missingAuthorization = observation.steps.find(item => item.authorization === null);
  if (missingAuthorization !== undefined) throw new Error(`Shadow evaluation Step ${missingAuthorization.stepNo} has no persisted authorization`);
  const retriedStep = observation.steps.find(item => item.attemptCount !== 1);
  if (retriedStep !== undefined) throw new Error(`Shadow evaluation Step ${retriedStep.stepNo} lacks per-attempt latency evidence`);
  const invalidCall = observation.modelCalls.find(item => !["completed", "failed", "abandoned"].includes(item.status));
  if (invalidCall !== undefined) throw new Error("Shadow evaluation model calls must be terminal");
  const invalidTool = observation.toolCalls.find(item => !["completed", "failed"].includes(item.status));
  if (invalidTool !== undefined) throw new Error("Shadow evaluation Tool calls must be terminal");
}

function evaluationTaskStatus(observation: ShadowEvalObservation): string {
  if (isTerminalStatus(observation.taskStatus)) return observation.taskStatus;
  const workflowStatus = observation.workflowStatus?.trim();
  if (observation.taskStatus === "running" && workflowStatus !== undefined && isTerminalStatus(workflowStatus)) {
    return workflowStatus;
  }
  return observation.taskStatus;
}

function isTerminalStatus(value: string): boolean {
  return ["completed", "failed", "cancelled"].includes(value);
}

function requiredTraceId(value: string): void {
  value = value.trim();
  if (value.length < 1 || value.length > 128 || !/^[A-Za-z0-9._:-]+$/.test(value)) {
    throw new Error("Shadow evaluation Trace ID is missing or invalid");
  }
}

function artifactId(value: ShadowEvalObservation["artifacts"][number]): string {
  return `artifact:${value.artifactType}:v${value.version}`;
}

export function evidenceId(provenance: { readonly sourceType: string; readonly sourceId: string }): string {
  const digest = createHash("sha256").update(`${provenance.sourceType}\u0000${provenance.sourceId}`, "utf8").digest("hex");
  return `evidence:${digest.slice(0, 32)}`;
}

function sourceBinding(taskId: string, runId: string): string {
  return createHash("sha256").update(`${taskId}\u0000${runId}`, "utf8").digest("hex").slice(0, 24);
}

function costMetrics(manifest: ShadowEvalManifest, observation: ShadowEvalObservation) {
  const prices = new Map(manifest.labels.cost.routePrices.map(item => [item.route, item]));
  let totalTokens = 0;
  let totalCost = 0n;
  let latencyMs = 0;
  for (const call of observation.modelCalls) {
    if (call.inputTokens === null || call.outputTokens === null || call.latencyMs === null) {
      throw new Error(`Shadow evaluation model call on route ${call.route} has incomplete metrics`);
    }
    const price = prices.get(call.route);
    if (price === undefined) throw new Error(`Shadow evaluation route price is missing for ${call.route}`);
    totalTokens += call.inputTokens + call.outputTokens;
    latencyMs += call.latencyMs;
    totalCost += BigInt(call.inputTokens) * BigInt(price.inputMicrousdPerMillionTokens);
    totalCost += BigInt(call.outputTokens) * BigInt(price.outputMicrousdPerMillionTokens);
  }
  for (const step of observation.steps) latencyMs += step.latencyMs!;
  for (const call of observation.toolCalls) {
    if (call.latencyMs === null) throw new Error("Shadow evaluation Tool call has incomplete latency");
    latencyMs += call.latencyMs;
  }
  const totalCostMicrousd = Number((totalCost + 999_999n) / 1_000_000n);
  if (!Number.isSafeInteger(totalCostMicrousd)) throw new Error("Shadow evaluation cost exceeds the safe integer range");
  return {
    modelCalls: observation.modelCalls.length,
    toolCalls: observation.steps.length + observation.toolCalls.length,
    totalTokens, totalCostMicrousd, latencyMs
  };
}
