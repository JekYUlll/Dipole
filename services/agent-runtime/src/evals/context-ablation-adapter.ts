import { z } from "zod";

import { parseContextAblationEvalSuite, type ContextAblationEvalSuite } from "./context-ablation-eval.js";
import { evidenceId, type ShadowEvalObservation } from "./shadow-eval-adapter.js";
import type { ContextAblationCaseObservation } from "./mysql-context-ablation-store.js";

const id = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:-]*$/);
const caseSha256 = z.string().regex(/^[a-f0-9]{64}$/);
const condition = z.enum(["baseline", "retrieval", "memory"]);
const routePrice = z.object({
  route: z.string().trim().min(1).max(255),
  inputMicrousdPerMillionTokens: z.number().int().nonnegative().safe(),
  outputMicrousdPerMillionTokens: z.number().int().nonnegative().safe()
}).strict();

const manifestSchema = z.object({
  schemaVersion: z.literal("dipole.agent.context-ablation-manifest.v1"),
  experimentId: id,
  candidateVersion: z.string().trim().min(2).max(128),
  routePrices: z.array(routePrice).min(1).max(64).refine(items => new Set(items.map(item => item.route)).size === items.length, "model routes must be unique"),
  cases: z.array(z.object({
    caseSha256,
    requiredOutputIds: z.array(id).min(1).max(32).refine(values => new Set(values).size === values.length, "output IDs must be unique"),
    relevantEvidenceIds: z.array(id).min(1).max(64).refine(values => new Set(values).size === values.length, "evidence IDs must be unique")
  }).strict()).min(1).max(256).refine(items => new Set(items.map(item => item.caseSha256)).size === items.length, "case SHA-256 values must be unique")
}).strict();

export type ContextAblationManifest = z.infer<typeof manifestSchema>;

export function parseContextAblationManifest(value: unknown): ContextAblationManifest {
  return manifestSchema.parse(typeof value === "string" ? JSON.parse(value) as unknown : value);
}

/**
 * Compiles reviewed labels and read-only Task/Run observations into the generic
 * ablation evaluator input. The manifest deliberately contains only stable IDs.
 */
export function buildContextAblationEvalSuite(
  rawManifest: unknown,
  observations: readonly ContextAblationCaseObservation[]
): ContextAblationEvalSuite {
  const manifest = parseContextAblationManifest(rawManifest);
  if (observations.length !== manifest.cases.length) throw new Error("Context ablation observation count does not match the manifest");
  const observed = new Map(observations.map(item => [item.caseSha256, item]));
  if (observed.size !== observations.length) throw new Error("Context ablation observations contain duplicate cases");
  const prices = new Map(manifest.routePrices.map(item => [item.route, item]));

  return parseContextAblationEvalSuite({
    schemaVersion: "dipole.agent.context-ablation-eval.v1",
    candidateVersion: manifest.candidateVersion,
    cases: manifest.cases.map(labels => {
      const entry = observed.get(labels.caseSha256);
      if (entry === undefined) throw new Error(`Context ablation observation is missing for case ${labels.caseSha256}`);
      if (entry.candidateVersion !== manifest.candidateVersion) throw new Error("Context ablation candidate version does not match the manifest");
      return {
        caseId: `case:${labels.caseSha256}`,
        requiredOutputIds: labels.requiredOutputIds,
        relevantEvidenceIds: labels.relevantEvidenceIds,
        results: condition.options.map(name => summarize(entry.observations[name], name, prices))
      };
    })
  });
}

function summarize(
  observation: ShadowEvalObservation,
  _condition: z.infer<typeof condition>,
  prices: ReadonlyMap<string, z.infer<typeof routePrice>>
) {
  assertObservation(observation);
  let totalTokens = 0;
  let totalCost = 0n;
  let latencyMs = 0;
  for (const call of observation.modelCalls) {
    const price = prices.get(call.route);
    if (price === undefined) throw new Error(`Context ablation route price is missing for ${call.route}`);
    totalTokens += call.inputTokens! + call.outputTokens!;
    totalCost += BigInt(call.inputTokens!) * BigInt(price.inputMicrousdPerMillionTokens);
    totalCost += BigInt(call.outputTokens!) * BigInt(price.outputMicrousdPerMillionTokens);
    latencyMs += call.latencyMs!;
  }
  for (const step of observation.steps) latencyMs += step.latencyMs!;
  for (const call of observation.toolCalls) latencyMs += call.latencyMs!;
  const totalCostMicrousd = Number((totalCost + 999_999n) / 1_000_000n);
  if (!Number.isSafeInteger(totalCostMicrousd)) throw new Error("Context ablation cost exceeds the safe integer range");

  return {
    condition: _condition,
    outputIds: observation.artifacts.map(item => `artifact:${item.artifactType}:v${item.version}`).sort(),
    evidenceIds: [...new Set(observation.contextManifest.selected
      .filter(item => !["runtime_policy", "execution_context", "agent_task", "capability_registry"].includes(item.provenance.sourceType))
      .map(item => evidenceId(item.provenance)))].sort(),
    allowed: true,
    metrics: {
      modelCalls: observation.modelCalls.length,
      toolCalls: observation.steps.length + observation.toolCalls.length,
      totalTokens,
      totalCostMicrousd,
      latencyMs
    }
  };
}

function assertObservation(observation: ShadowEvalObservation): void {
  if (!isTerminal(evaluationTaskStatus(observation)) || !isTerminal(observation.runStatus)) throw new Error("Context ablation observation must be terminal");
  if (!/^[A-Za-z0-9._:-]{1,128}$/.test(observation.traceId)) throw new Error("Context ablation observation trace ID is invalid");
  if (observation.steps.length > 256 || observation.artifacts.length > 256 || observation.modelCalls.length > 64 || observation.toolCalls.length > 256) throw new Error("Context ablation observation exceeds bounded collection limits");
  for (const step of observation.steps) {
    if (!["completed", "failed", "denied"].includes(step.status) || step.latencyMs === null || step.authorization === null || step.attemptCount !== 1) throw new Error(`Context ablation Step ${step.stepNo} lacks complete authorization evidence`);
  }
  for (const call of observation.modelCalls) {
    if (!["completed", "failed", "abandoned"].includes(call.status) || call.inputTokens === null || call.outputTokens === null || call.latencyMs === null) throw new Error("Context ablation model call lacks complete metering");
  }
  for (const call of observation.toolCalls) {
    if (!["completed", "failed"].includes(call.status) || call.latencyMs === null) throw new Error("Context ablation Tool call lacks complete latency");
  }
}

function evaluationTaskStatus(observation: ShadowEvalObservation): string {
  if (isTerminal(observation.taskStatus)) return observation.taskStatus;
  return observation.taskStatus === "running" && observation.workflowStatus !== null && observation.workflowStatus !== undefined && isTerminal(observation.workflowStatus)
    ? observation.workflowStatus
    : observation.taskStatus;
}

function isTerminal(value: string): boolean { return ["completed", "failed", "cancelled"].includes(value); }
