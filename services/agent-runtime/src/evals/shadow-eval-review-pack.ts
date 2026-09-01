import { createHash } from "node:crypto";

import { evidenceId, type ShadowEvalObservation } from "./shadow-eval-adapter.js";

export interface ShadowEvalReviewPack {
  readonly schemaVersion: "dipole.agent.shadow-eval-review-pack.v1";
  readonly reviewStatus: "review_required";
  readonly candidateVersion: string;
  readonly evaluatorEligibility: {
    readonly status: "eligible" | "blocked";
    readonly blockingReasons: readonly string[];
  };
  readonly binding: {
    readonly taskFingerprint: string;
    readonly runFingerprint: string;
    readonly traceFingerprint: string;
  };
  readonly observed: {
    readonly execution: { readonly taskStatus: string; readonly workflowStatus: string | null; readonly runStatus: string };
    readonly outputIds: readonly string[];
    readonly trajectory: readonly string[];
    readonly permissions: readonly {
      readonly stepNo: number;
      readonly capabilityId: string;
      readonly status: string;
      readonly authorization: {
        readonly status: "complete";
        readonly resourceType: string;
        readonly resourceFingerprint: string;
        readonly action: string;
        readonly decision: "allowed";
      } | { readonly status: "missing" };
    }[];
    readonly retrievedEvidenceIds: readonly string[];
    readonly metering: {
      readonly modelCalls: readonly { readonly route: string; readonly status: string; readonly tokenMetering: "complete" | "unavailable"; readonly latencyMetering: "complete" | "unavailable" }[];
      readonly toolCallCount: number;
      readonly stepCount: number;
    };
  };
  readonly reviewerChecklist: readonly string[];
}

/**
 * Produces a low-sensitivity observation packet for an independent reviewer.
 * The packet cannot be consumed by the evaluator: review creates the final,
 * Task/Run-bound manifest in a controlled workspace.
 */
export function buildShadowEvalReviewPack(candidateVersion: string, observation: ShadowEvalObservation): ShadowEvalReviewPack {
  candidateVersion = requiredCandidateVersion(candidateVersion);
  assertExportable(observation);

  const terminalTaskStatus = evaluationTaskStatus(observation);
  const outputIds = [
    ...observation.artifacts.map(artifact => `artifact:${artifact.artifactType}:v${artifact.version}`),
    `run:${observation.runStatus}`,
    `task:${terminalTaskStatus}`
  ].sort();
  const trajectory = [
    ...(observation.contextManifest.selected.length > 0 ? ["context.compile"] : []),
    ...observation.steps.map(step => `capability:${step.capabilityId}:${step.status}`),
    ...observation.artifacts.map(artifact => `artifact:${artifact.artifactType}:v${artifact.version}`)
  ];
  const retrievedEvidenceIds = [...new Set(observation.contextManifest.selected
    .filter(item => isRetrievedEvidence(item.provenance.sourceType))
    .map(item => evidenceId(item.provenance)))].sort();

  return {
    schemaVersion: "dipole.agent.shadow-eval-review-pack.v1",
    reviewStatus: "review_required",
    candidateVersion,
    evaluatorEligibility: evaluatorEligibility(observation),
    binding: {
      taskFingerprint: fingerprint("task", observation.taskId),
      runFingerprint: fingerprint("run", observation.runId),
      traceFingerprint: fingerprint("trace", observation.traceId)
    },
    observed: {
      execution: {
        taskStatus: terminalTaskStatus,
        workflowStatus: observation.workflowStatus ?? null,
        runStatus: observation.runStatus
      },
      outputIds,
      trajectory,
      permissions: observation.steps.map(step => ({
        stepNo: step.stepNo,
        capabilityId: step.capabilityId,
        status: step.status,
        authorization: step.authorization === null ? { status: "missing" } : {
          status: "complete",
          resourceType: step.authorization.resourceType,
          resourceFingerprint: fingerprint("resource", step.authorization.resourceId),
          action: step.authorization.action,
          decision: step.authorization.decision
        }
      })),
      retrievedEvidenceIds,
      metering: {
        modelCalls: observation.modelCalls.map(call => ({
          route: call.route,
          status: call.status,
          tokenMetering: call.inputTokens === null || call.outputTokens === null ? "unavailable" : "complete",
          latencyMetering: call.latencyMs === null ? "unavailable" : "complete"
        })),
        toolCallCount: observation.toolCalls.length,
        stepCount: observation.steps.length
      }
    },
    reviewerChecklist: [
      "Create a separate manifest bound to the original Task and Run in a controlled review workspace.",
      "Independently set expected outcome, trajectory, authorization, retrieval and cost labels.",
      "Record route prices and metric ceilings from the reviewed candidate policy; this packet never proposes them.",
      "Treat this packet as observation only. It does not approve a candidate or establish an Agent success rate."
    ]
  };
}

function assertExportable(observation: ShadowEvalObservation): void {
  if (!isTerminalStatus(evaluationTaskStatus(observation)) || !isTerminalStatus(observation.runStatus)) {
    throw new Error("Shadow evaluation review pack requires terminal Task execution and Run");
  }
  requiredBindingValue(observation.taskId, "Task ID");
  requiredBindingValue(observation.runId, "Run ID");
  requiredBindingValue(observation.traceId, "Trace ID");
  if (observation.steps.length > 256 || observation.artifacts.length > 256 || observation.toolCalls.length > 256 || observation.modelCalls.length > 64) {
    throw new Error("Shadow evaluation review pack exceeds its bounded collection limits");
  }
}

function evaluatorEligibility(observation: ShadowEvalObservation): ShadowEvalReviewPack["evaluatorEligibility"] {
  const blockingReasons: string[] = [];
  for (const step of observation.steps) {
    if (!isTerminalStepStatus(step.status)) blockingReasons.push(`step_${step.stepNo}_non_terminal`);
    if (step.latencyMs === null) blockingReasons.push(`step_${step.stepNo}_missing_latency`);
    if (step.authorization === null) blockingReasons.push(`step_${step.stepNo}_missing_authorization`);
    if (step.attemptCount !== 1) blockingReasons.push(`step_${step.stepNo}_attempt_evidence_incomplete`);
  }
  for (const [index, call] of observation.modelCalls.entries()) {
    if (!["completed", "failed", "abandoned"].includes(call.status)) blockingReasons.push(`model_call_${index + 1}_non_terminal`);
    if (call.latencyMs === null) blockingReasons.push(`model_call_${index + 1}_missing_latency`);
  }
  for (const [index, call] of observation.toolCalls.entries()) {
    if (!["completed", "failed"].includes(call.status)) blockingReasons.push(`tool_call_${index + 1}_non_terminal`);
    if (call.latencyMs === null) blockingReasons.push(`tool_call_${index + 1}_missing_latency`);
  }
  return { status: blockingReasons.length === 0 ? "eligible" : "blocked", blockingReasons };
}

function requiredBindingValue(value: string, label: string): void {
  if (value.trim().length === 0 || value.length > 128) throw new Error(`Shadow evaluation review pack ${label} is invalid`);
}

function requiredCandidateVersion(value: string): string {
  value = value.trim();
  if (value.length < 2 || value.length > 128 || !/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/.test(value)) {
    throw new Error("Shadow evaluation review pack candidate version is invalid");
  }
  return value;
}

function evaluationTaskStatus(observation: ShadowEvalObservation): string {
  if (isTerminalStatus(observation.taskStatus)) return observation.taskStatus;
  const workflowStatus = observation.workflowStatus?.trim();
  if (observation.taskStatus === "running" && workflowStatus !== undefined && isTerminalStatus(workflowStatus)) return workflowStatus;
  return observation.taskStatus;
}

function isTerminalStatus(value: string): boolean {
  return ["completed", "failed", "cancelled"].includes(value);
}

function isTerminalStepStatus(value: string): boolean {
  return ["completed", "failed", "denied"].includes(value);
}

function isRetrievedEvidence(sourceType: string): boolean {
  return !new Set(["runtime_policy", "execution_context", "agent_task", "capability_registry"]).has(sourceType);
}

function fingerprint(kind: string, value: string): string {
  return `sha256:${createHash("sha256").update(`dipole.agent.shadow-eval-review-pack.v1\u0000${kind}\u0000${value}`, "utf8").digest("hex")}`;
}
