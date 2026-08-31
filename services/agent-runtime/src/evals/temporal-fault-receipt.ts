import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";

const schemaVersion = "dipole.agent.temporal-fault-receipt.v1" as const;
const inputSchemaVersion = "dipole.agent.temporal-fault-observation.v1" as const;
const identity = z.string().trim().min(1).max(128);
const timestamp = z.string().datetime({ offset: true });
const status = z.enum(["running", "waiting_approval", "waiting_input", "completed", "failed", "cancelled"]);

const observationSchema = z.object({
  schemaVersion: z.literal(inputSchemaVersion),
  drillId: z.literal("worker_replacement_approval_resume"),
  observedAt: timestamp,
  workflow: z.object({ taskId: identity, runId: identity }).strict(),
  transitions: z.array(z.object({ revision: z.number().int().min(1).max(256), status }).strict()).min(1).max(16),
  faults: z.object({ workerReplacements: z.number().int().min(0).max(8), terminalWriteRetries: z.number().int().min(0).max(8) }).strict(),
  effects: z.object({
    admissions: z.number().int().min(0).max(16), stepExecutions: z.number().int().min(0).max(256),
    approvalRequests: z.number().int().min(0).max(16), approvalResolutions: z.number().int().min(0).max(16),
    terminalWriteAttempts: z.number().int().min(0).max(16), terminalPersistedWrites: z.number().int().min(0).max(16)
  }).strict()
}).strict();

export interface TemporalFaultObservation {
  readonly schemaVersion: typeof inputSchemaVersion;
  readonly drillId: "worker_replacement_approval_resume";
  readonly observedAt: string;
  readonly workflow: { readonly taskId: string; readonly runId: string };
  readonly transitions: ReadonlyArray<{ readonly revision: number; readonly status: z.infer<typeof status> }>;
  readonly faults: { readonly workerReplacements: number; readonly terminalWriteRetries: number };
  readonly effects: {
    readonly admissions: number; readonly stepExecutions: number; readonly approvalRequests: number;
    readonly approvalResolutions: number; readonly terminalWriteAttempts: number; readonly terminalPersistedWrites: number;
  };
}

export interface TemporalFaultReceipt {
  readonly schemaVersion: typeof schemaVersion;
  readonly receiptId: string;
  readonly receiptSha256: string;
  readonly drillId: TemporalFaultObservation["drillId"];
  readonly observedAt: string;
  readonly workflow: TemporalFaultObservation["workflow"];
  readonly outcome: "eligible" | "ineligible";
  readonly transitions: TemporalFaultObservation["transitions"];
  readonly faults: TemporalFaultObservation["faults"];
  readonly effects: TemporalFaultObservation["effects"];
  readonly failures: readonly string[];
}

export function createTemporalFaultReceipt(raw: unknown): TemporalFaultReceipt {
  const observation = observationSchema.parse(raw);
  const failures = validateWorkerReplacementApprovalResume(observation);
  const body = {
    schemaVersion, drillId: observation.drillId, observedAt: observation.observedAt, workflow: observation.workflow,
    outcome: failures.length === 0 ? "eligible" as const : "ineligible" as const,
    transitions: observation.transitions, faults: observation.faults, effects: observation.effects, failures
  };
  const receiptSha256 = sha256(body);
  return { ...body, receiptSha256, receiptId: `TEMPORAL-FAULT-${sha256({ ...body, receiptSha256 })}` };
}

export function validateTemporalFaultReceipt(raw: unknown): TemporalFaultReceipt {
  const receipt = z.object({
    schemaVersion: z.literal(schemaVersion), receiptId: z.string().regex(/^TEMPORAL-FAULT-[a-f0-9]{64}$/), receiptSha256: z.string().regex(/^[a-f0-9]{64}$/),
    drillId: z.literal("worker_replacement_approval_resume"), observedAt: timestamp,
    workflow: z.object({ taskId: identity, runId: identity }).strict(),
    outcome: z.enum(["eligible", "ineligible"]),
    transitions: observationSchema.shape.transitions, faults: observationSchema.shape.faults, effects: observationSchema.shape.effects,
    failures: z.array(z.string().min(1).max(256)).max(16)
  }).strict().parse(raw);
  const expected = createTemporalFaultReceipt({
    schemaVersion: inputSchemaVersion, drillId: receipt.drillId, observedAt: receipt.observedAt, workflow: receipt.workflow,
    transitions: receipt.transitions, faults: receipt.faults, effects: receipt.effects
  });
  if (canonicalMcpJSON(receipt) !== canonicalMcpJSON(expected)) throw new Error("Temporal fault receipt binding is invalid");
  return expected;
}

function validateWorkerReplacementApprovalResume(observation: z.infer<typeof observationSchema>): string[] {
  const failures: string[] = [];
  const transitionKey = observation.transitions.map(item => `${item.revision}:${item.status}`).join(",");
  if (transitionKey !== "1:running,2:waiting_approval,3:running,4:completed") failures.push("unexpected_state_transitions");
  if (observation.faults.workerReplacements !== 1) failures.push("worker_replacement_count_mismatch");
  if (observation.faults.terminalWriteRetries !== 1) failures.push("terminal_write_retry_count_mismatch");
  const effects = observation.effects;
  if (effects.admissions !== 1) failures.push("admission_count_mismatch");
  if (effects.stepExecutions !== 4) failures.push("step_execution_count_mismatch");
  if (effects.approvalRequests !== 1 || effects.approvalResolutions !== 1) failures.push("approval_side_effect_count_mismatch");
  if (effects.terminalWriteAttempts !== 2 || effects.terminalPersistedWrites !== 1) failures.push("terminal_write_side_effect_count_mismatch");
  return failures;
}

function sha256(value: unknown): string {
  return createHash("sha256").update(canonicalMcpJSON(value), "utf8").digest("hex");
}
