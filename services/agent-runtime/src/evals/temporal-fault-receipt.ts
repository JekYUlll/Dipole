import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";

const schemaVersion = "dipole.agent.temporal-fault-receipt.v1" as const;
const inputSchemaVersion = "dipole.agent.temporal-fault-observation.v1" as const;
const identity = z.string().trim().min(1).max(128);
const timestamp = z.string().datetime({ offset: true });
const status = z.enum(["running", "waiting_approval", "waiting_input", "completed", "failed", "cancelled"]);

const drillIdSchema = z.enum([
  "worker_replacement_approval_resume",
  "worker_replacement_input_resume",
  "read_scope_confirmation_resume",
  "read_scope_confirmation_declined",
  "read_scope_confirmation_expired"
]);
const cancellationReason = z.enum(["input_expired", "user_cancelled"]);

const observationSchema = z.object({
  schemaVersion: z.literal(inputSchemaVersion),
  drillId: drillIdSchema,
  observedAt: timestamp,
  workflow: z.object({ taskId: identity, runId: identity }).strict(),
  transitions: z.array(z.object({ revision: z.number().int().min(1).max(256), status }).strict()).min(1).max(16),
  faults: z.object({ workerReplacements: z.number().int().min(0).max(8), terminalWriteRetries: z.number().int().min(0).max(8) }).strict(),
  effects: z.object({
    admissions: z.number().int().min(0).max(16), stepExecutions: z.number().int().min(0).max(256),
    approvalRequests: z.number().int().min(0).max(16), approvalResolutions: z.number().int().min(0).max(16),
    terminalWriteAttempts: z.number().int().min(0).max(16), terminalPersistedWrites: z.number().int().min(0).max(16),
    inputSignalsRejected: z.number().int().min(0).max(16), inputResumptions: z.number().int().min(0).max(16),
    // Read scope drills only; receipts archived before those drills stay valid without them.
    conversationReads: z.number().int().min(0).max(16).optional(),
    unconfirmedConversationReads: z.number().int().min(0).max(16).optional()
  }).strict(),
  cancellation: z.object({ reason: cancellationReason }).strict().optional()
}).strict();

export interface TemporalFaultObservation {
  readonly schemaVersion: typeof inputSchemaVersion;
  readonly drillId: z.infer<typeof drillIdSchema>;
  readonly observedAt: string;
  readonly workflow: { readonly taskId: string; readonly runId: string };
  readonly transitions: ReadonlyArray<{ readonly revision: number; readonly status: z.infer<typeof status> }>;
  readonly faults: { readonly workerReplacements: number; readonly terminalWriteRetries: number };
  readonly effects: {
    readonly admissions: number; readonly stepExecutions: number; readonly approvalRequests: number;
    readonly approvalResolutions: number; readonly terminalWriteAttempts: number; readonly terminalPersistedWrites: number;
    readonly inputSignalsRejected: number; readonly inputResumptions: number;
    readonly conversationReads?: number | undefined; readonly unconfirmedConversationReads?: number | undefined;
  };
  readonly cancellation?: { readonly reason: z.infer<typeof cancellationReason> } | undefined;
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
  readonly cancellation?: TemporalFaultObservation["cancellation"];
  readonly failures: readonly string[];
}

export function createTemporalFaultReceipt(raw: unknown): TemporalFaultReceipt {
  const observation = observationSchema.parse(raw);
  const failures = validateObservation(observation);
  const body = {
    schemaVersion, drillId: observation.drillId, observedAt: observation.observedAt, workflow: observation.workflow,
    outcome: failures.length === 0 ? "eligible" as const : "ineligible" as const,
    transitions: observation.transitions, faults: observation.faults, effects: observation.effects,
    ...(observation.cancellation === undefined ? {} : { cancellation: observation.cancellation }),
    failures
  };
  const receiptSha256 = sha256(body);
  return { ...body, receiptSha256, receiptId: `TEMPORAL-FAULT-${sha256({ ...body, receiptSha256 })}` };
}

export function validateTemporalFaultReceipt(raw: unknown): TemporalFaultReceipt {
  const receipt = z.object({
    schemaVersion: z.literal(schemaVersion), receiptId: z.string().regex(/^TEMPORAL-FAULT-[a-f0-9]{64}$/), receiptSha256: z.string().regex(/^[a-f0-9]{64}$/),
    drillId: drillIdSchema, observedAt: timestamp,
    workflow: z.object({ taskId: identity, runId: identity }).strict(),
    outcome: z.enum(["eligible", "ineligible"]),
    transitions: observationSchema.shape.transitions, faults: observationSchema.shape.faults, effects: observationSchema.shape.effects,
    cancellation: observationSchema.shape.cancellation,
    failures: z.array(z.string().min(1).max(256)).max(16)
  }).strict().parse(raw);
  const expected = createTemporalFaultReceipt({
    schemaVersion: inputSchemaVersion, drillId: receipt.drillId, observedAt: receipt.observedAt, workflow: receipt.workflow,
    transitions: receipt.transitions, faults: receipt.faults, effects: receipt.effects,
    ...(receipt.cancellation === undefined ? {} : { cancellation: receipt.cancellation })
  });
  if (canonicalMcpJSON(receipt) !== canonicalMcpJSON(expected)) throw new Error("Temporal fault receipt binding is invalid");
  return expected;
}

function validateObservation(observation: z.infer<typeof observationSchema>): string[] {
  switch (observation.drillId) {
    case "worker_replacement_approval_resume":
      return validateWorkerReplacementApprovalResume(observation);
    case "worker_replacement_input_resume":
      return validateWorkerReplacementInputResume(observation);
    case "read_scope_confirmation_resume":
      return validateReadScopeConfirmationResume(observation);
    case "read_scope_confirmation_declined":
      return validateReadScopeConfirmationTerminated(observation, "user_cancelled");
    case "read_scope_confirmation_expired":
      return validateReadScopeConfirmationTerminated(observation, "input_expired");
  }
}

function validateReadScopeConfirmationResume(observation: z.infer<typeof observationSchema>): string[] {
  const failures = validateReadScopeBaseline(observation);
  if (transitionKey(observation) !== "1:running,2:waiting_input,3:running,4:completed") failures.push("unexpected_state_transitions");
  if (observation.cancellation !== undefined) failures.push("unexpected_cancellation");
  const effects = observation.effects;
  // Two Step executions: the paused discovery Step and the confirmed read Step.
  if (effects.stepExecutions !== 2) failures.push("step_execution_count_mismatch");
  if (effects.inputResumptions !== 1) failures.push("input_resumption_count_mismatch");
  if (effects.conversationReads !== 1) failures.push("confirmed_read_count_mismatch");
  return failures;
}

function validateReadScopeConfirmationTerminated(
  observation: z.infer<typeof observationSchema>,
  expectedReason: z.infer<typeof cancellationReason>
): string[] {
  const failures = validateReadScopeBaseline(observation);
  if (transitionKey(observation) !== "1:running,2:waiting_input,3:cancelled") failures.push("unexpected_state_transitions");
  if (observation.cancellation?.reason !== expectedReason) failures.push("unexpected_cancellation_reason");
  const effects = observation.effects;
  if (effects.stepExecutions !== 1) failures.push("step_execution_count_mismatch");
  if (effects.inputResumptions !== 0) failures.push("unexpected_input_resumption");
  if (effects.conversationReads !== 0) failures.push("unexpected_conversation_read");
  return failures;
}

function validateReadScopeBaseline(observation: z.infer<typeof observationSchema>): string[] {
  const failures: string[] = [];
  if (observation.faults.workerReplacements !== 0 || observation.faults.terminalWriteRetries !== 0) failures.push("unexpected_injected_faults");
  const effects = observation.effects;
  if (effects.admissions !== 1) failures.push("admission_count_mismatch");
  if (effects.approvalRequests !== 0 || effects.approvalResolutions !== 0) failures.push("unexpected_approval_side_effects");
  if (effects.terminalWriteAttempts !== 1 || effects.terminalPersistedWrites !== 1) failures.push("terminal_write_side_effect_count_mismatch");
  if (effects.conversationReads === undefined || effects.unconfirmedConversationReads === undefined) failures.push("missing_read_scope_evidence");
  if (effects.unconfirmedConversationReads !== 0) failures.push("unconfirmed_conversation_read");
  return failures;
}

function transitionKey(observation: z.infer<typeof observationSchema>): string {
  return observation.transitions.map(item => `${item.revision}:${item.status}`).join(",");
}

function validateWorkerReplacementApprovalResume(observation: z.infer<typeof observationSchema>): string[] {
  const failures: string[] = [];
  if (transitionKey(observation) !== "1:running,2:waiting_approval,3:running,4:completed") failures.push("unexpected_state_transitions");
  if (observation.faults.workerReplacements !== 1) failures.push("worker_replacement_count_mismatch");
  if (observation.faults.terminalWriteRetries !== 1) failures.push("terminal_write_retry_count_mismatch");
  const effects = observation.effects;
  if (effects.admissions !== 1) failures.push("admission_count_mismatch");
  if (effects.stepExecutions !== 4) failures.push("step_execution_count_mismatch");
  if (effects.approvalRequests !== 1 || effects.approvalResolutions !== 1) failures.push("approval_side_effect_count_mismatch");
  if (effects.terminalWriteAttempts !== 2 || effects.terminalPersistedWrites !== 1) failures.push("terminal_write_side_effect_count_mismatch");
  if (effects.inputSignalsRejected !== 0 || effects.inputResumptions !== 0) failures.push("unexpected_input_side_effects");
  return failures;
}

function validateWorkerReplacementInputResume(observation: z.infer<typeof observationSchema>): string[] {
  const failures: string[] = [];
  if (transitionKey(observation) !== "1:running,2:waiting_input,3:running,4:completed") failures.push("unexpected_state_transitions");
  if (observation.faults.workerReplacements !== 1 || observation.faults.terminalWriteRetries !== 0) failures.push("fault_count_mismatch");
  const effects = observation.effects;
  if (effects.admissions !== 1 || effects.stepExecutions !== 2) failures.push("task_side_effect_count_mismatch");
  if (effects.approvalRequests !== 0 || effects.approvalResolutions !== 0) failures.push("unexpected_approval_side_effects");
  if (effects.inputSignalsRejected !== 2 || effects.inputResumptions !== 1) failures.push("input_side_effect_count_mismatch");
  if (effects.terminalWriteAttempts !== 1 || effects.terminalPersistedWrites !== 1) failures.push("terminal_write_side_effect_count_mismatch");
  return failures;
}

function sha256(value: unknown): string {
  return createHash("sha256").update(canonicalMcpJSON(value), "utf8").digest("hex");
}
