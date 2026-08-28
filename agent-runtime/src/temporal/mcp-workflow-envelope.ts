import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";
import type {
  TemporalMcpDispatchActivityInput,
  TemporalMcpDispatchCheckpointV1,
  TemporalMcpDispatchRouteBinding
} from "./mcp-dispatch-activity.js";
import type { AgentTaskResume } from "../task/agent-task-state.js";

const identitySchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/);
const coreIdentitySchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$/);
const routeBindingSchema = z.object({
  routeId: identitySchema,
  routeVersion: z.number().int().min(1).max(1_000_000),
  routeManifestSha256: z.string().regex(/^[a-f0-9]{64}$/)
}).strict();
const executionSchema = routeBindingSchema.extend({
  kind: z.literal("external_mcp_v1"),
  arguments: z.record(z.string(), z.unknown())
}).strict();
const activityIdentitySchema = z.object({
  taskId: coreIdentitySchema,
  runId: coreIdentitySchema,
  principalUserId: identitySchema,
  requestId: identitySchema.optional(),
  traceId: identitySchema.optional()
}).strict();

export interface TemporalMcpWorkflowExecutionV1 extends TemporalMcpDispatchRouteBinding {
  readonly kind: "external_mcp_v1";
  readonly arguments: Readonly<Record<string, unknown>>;
}

export class TemporalMcpWorkflowExecutionCatalog {
  readonly #bindings = new Map<string, TemporalMcpDispatchRouteBinding>();

  constructor(bindings: readonly TemporalMcpDispatchRouteBinding[]) {
    if (bindings.length === 0) throw new Error("Temporal MCP Workflow routes are unavailable");
    for (const rawBinding of bindings) {
      const binding = routeBindingSchema.parse(rawBinding);
      if (this.#bindings.has(binding.routeId)) {
        throw new Error("Temporal MCP Workflow route ID is duplicated");
      }
      this.#bindings.set(binding.routeId, binding);
    }
  }

  create(routeId: string, rawArguments: unknown): TemporalMcpWorkflowExecutionV1 {
    const binding = this.#bindings.get(routeId.trim());
    if (binding === undefined) throw new Error("Temporal MCP Workflow route is unavailable");
    return validateTemporalMcpWorkflowExecution({
      kind: "external_mcp_v1",
      ...binding,
      arguments: canonicalArguments(rawArguments)
    });
  }
}

export function validateTemporalMcpWorkflowExecution(raw: unknown): TemporalMcpWorkflowExecutionV1 {
  const parsed = executionSchema.safeParse(raw);
  if (!parsed.success) throw new Error("Temporal MCP Workflow execution envelope is invalid");
  return {
    kind: "external_mcp_v1",
    routeId: parsed.data.routeId,
    routeVersion: parsed.data.routeVersion,
    routeManifestSha256: parsed.data.routeManifestSha256,
    arguments: parsed.data.arguments
  };
}

export function createTemporalMcpBeginActivityInput(
  rawExecution: unknown,
  rawIdentity: {
    readonly taskId: string;
    readonly runId: string;
    readonly principalUserId: string;
    readonly requestId?: string;
    readonly traceId?: string;
  }
): Extract<TemporalMcpDispatchActivityInput, { kind: "begin" }> {
  const execution = validateTemporalMcpWorkflowExecution(rawExecution);
  const identity = activityIdentitySchema.parse(rawIdentity);
  return {
    kind: "begin",
    routeId: execution.routeId,
    routeVersion: execution.routeVersion,
    routeManifestSha256: execution.routeManifestSha256,
    taskId: identity.taskId,
    runId: identity.runId,
    principalUserId: identity.principalUserId,
    arguments: execution.arguments,
    ...(identity.requestId === undefined ? {} : { requestId: identity.requestId }),
    ...(identity.traceId === undefined ? {} : { traceId: identity.traceId })
  };
}

export function createTemporalMcpResumeActivityInput(
  checkpoint: unknown,
  resume: AgentTaskResume | undefined
): Extract<TemporalMcpDispatchActivityInput, { kind: "resume" }> {
  if (checkpoint === undefined || resume === undefined) {
    throw new Error("Temporal MCP Workflow resume state is incomplete");
  }
  return {
    kind: "resume",
    checkpoint: checkpoint as TemporalMcpDispatchCheckpointV1,
    resume
  };
}

function canonicalArguments(raw: unknown): Readonly<Record<string, unknown>> {
  if (typeof raw !== "object" || raw === null || Array.isArray(raw)) {
    throw new Error("Temporal MCP Workflow arguments must be a JSON object");
  }
  const canonical = canonicalMcpJSON(raw);
  if (new TextEncoder().encode(canonical).byteLength > 16 * 1024) {
    throw new Error("Temporal MCP Workflow arguments exceed 16 KiB");
  }
  return JSON.parse(canonical) as Readonly<Record<string, unknown>>;
}
