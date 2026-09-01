import { createHash } from "node:crypto";

import { z } from "zod";

import { executionContextSchema } from "../runtime/execution-context.js";
import {
  agentEventSchema,
  agentRunId,
  agentTaskId,
  discoveredConversationMarker,
  executeShadowPlan,
  maxReadScopeOptions,
  type ShadowPlan,
  type ShadowPlanExecutionDependencies
} from "../events/shadow-processor.js";
import type { AgentTaskActivities, AgentTaskActivityInput } from "./agent-task-activities.js";
import type { AgentArtifactCreateInput, AgentArtifactRecord } from "../capabilities/agent-capability-rpc.js";
import { AgentTelemetry } from "../observability/agent-telemetry.js";
import type { AgentRuntimeMode } from "../capabilities/agent-capability-rpc.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import { agentElicitationSchemaVersion, validateElicitationForm, type AgentElicitationForm } from "../task/agent-elicitation.js";

const readScopeConfirmationKind = "dipole.agent.read-scope-confirmation.v1";
const readScopeFieldId = "conversation";
const defaultReadScopeConfirmationTtlMs = 15 * 60_000;

const readScopeCheckpointSchema = z.object({
  kind: z.literal(readScopeConfirmationKind),
  stepNo: z.number().int().positive(),
  candidates: z.array(z.string().trim().min(1).max(128)).min(2).max(maxReadScopeOptions),
  discoveredCount: z.number().int().positive(),
  planSummary: z.string().trim().min(1).max(2000),
  readLimit: z.number().int().min(1).max(100).optional()
}).strict();

type ReadScopeCheckpoint = z.infer<typeof readScopeCheckpointSchema>;

interface AgentArtifactWriter {
  createArtifact(input: AgentArtifactCreateInput): Promise<AgentArtifactRecord>;
}

interface AgentContextResolver {
  resolveMcpContext(taskId: string, runId: string, principalUserId: string, context?: { requestId?: string; traceId?: string }): Promise<ExecutionContext>;
}

export function createTemporalReadStepActivities(
  dependencies: ShadowPlanExecutionDependencies & {
    readonly artifacts?: AgentArtifactWriter;
    readonly runtimeMode?: AgentRuntimeMode;
    readonly contextResolver?: AgentContextResolver;
    readonly readPermissions?: readonly string[];
    readonly readScopeConfirmationTtlMs?: number;
  }
): AgentTaskActivities {
  return {
    async executeAgentTaskStep(input) {
      if (input.admission === undefined || input.shadowEvent === undefined) {
        throw new Error("Temporal read Step requires trusted admission and shadow event");
      }
      if (input.step !== 0 && input.step !== 1) {
        throw new Error("Temporal read Step supports the discovery Step and one owner-confirmed read Step");
      }
      const event = agentEventSchema.parse(input.shadowEvent);
      const admission = input.admission;
      const expectedTaskId = agentTaskId({
        tenantId: admission.tenantId,
        agentUuid: admission.agentId,
        triggerType: event.eventType,
        triggerRef: event.aggregateId
      });
      const runtimeMode = dependencies.runtimeMode ?? "shadow";
      if (input.taskId !== expectedTaskId || input.runId !== agentRunId(expectedTaskId, "dipole-agent", runtimeMode) ||
          admission.eventId !== event.eventId || admission.triggerType !== event.eventType ||
          admission.triggerRef !== event.aggregateId) {
        throw new Error("Temporal read Step Task, Run, admission, and event binding mismatch");
      }
      if (runtimeMode === "active" && dependencies.contextResolver === undefined) {
        throw new Error("Active Temporal read Step requires the Core Context resolver");
      }
      const context = dependencies.contextResolver === undefined
        ? executionContextSchema.parse({
          tenantId: admission.tenantId,
          principalUuid: admission.principalUserId,
          agentUuid: admission.agentId,
          taskId: input.taskId,
          runId: input.runId,
          mode: runtimeMode,
          permissions: dependencies.readPermissions ?? ["conversation.list", "conversation.read"],
          resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read", "list"] }],
          approvedCapabilities: [],
          eventId: event.eventId,
          ...(admission.requestId === undefined ? {} : { requestId: admission.requestId }),
          ...(admission.traceId === undefined ? {} : { traceId: admission.traceId })
        })
        : await dependencies.contextResolver.resolveMcpContext(input.taskId, input.runId, admission.principalUserId, {
          ...(admission.requestId === undefined ? {} : { requestId: admission.requestId }),
          ...(admission.traceId === undefined ? {} : { traceId: admission.traceId })
        });
      if (context.mode !== runtimeMode || context.taskId !== input.taskId || context.runId !== input.runId || context.eventId !== event.eventId) {
        throw new Error("Core Context binding mismatch for Temporal read Step");
      }
      const confirmation = input.step === 0 ? undefined : resolveReadScopeConfirmation(input, context);
      const telemetry = dependencies.telemetry ?? new AgentTelemetry();
      return telemetry.withSpan("agent.run", {
        taskId: context.taskId, runId: context.runId,
        attributes: { "dipole.agent.mode": context.mode, "dipole.agent.event.type": event.eventType }
      }, async span => {
        const execution = await executeShadowPlan(event, context, { ...dependencies, telemetry }, confirmation === undefined
          ? { confirmationRequired: true }
          : { confirmedConversationId: confirmation.conversationId, resumedPlan: confirmation.plan });
        if (execution.outcome === "awaiting_read_scope") {
          if (confirmation !== undefined) {
            throw new Error("Owner-confirmed Temporal read Step must not request another read scope confirmation");
          }
          span.setAttribute("dipole.agent.run.read_scope", "awaiting_confirmation");
          const checkpoint: ReadScopeCheckpoint = {
            kind: readScopeConfirmationKind,
            stepNo: execution.stepNo,
            candidates: [...execution.candidates],
            discoveredCount: execution.discoveredCount,
            planSummary: execution.plan.summary,
            ...readLimitOf(execution.plan)
          };
          return {
            kind: "wait_input",
            requestId: readScopeRequestId(context.taskId, context.runId, execution.stepNo),
            prompt: readScopePrompt(execution.candidates.length, execution.discoveredCount),
            form: readScopeForm(execution.candidates),
            source: { kind: "agent" },
            expiresAtUnixMs: Date.now() + (dependencies.readScopeConfirmationTtlMs ?? defaultReadScopeConfirmationTtlMs),
            checkpoint
          };
        }
        const plan = execution.plan;
        const artifact = dependencies.artifacts === undefined ? undefined : await telemetry.withSpan("agent.artifact.create", {
          taskId: context.taskId, runId: context.runId,
          attributes: { "dipole.agent.artifact.type": "conversation_digest", "dipole.agent.artifact.version": 1 }
        }, async artifactSpan => {
          const value = await dependencies.artifacts!.createArtifact({
            tenantId: context.tenantId,
            taskId: context.taskId,
            runId: context.runId,
            artifactType: "conversation_digest",
            version: 1,
            title: "Conversation digest",
            mediaType: "text/markdown",
            content: Buffer.from(`# Conversation digest\n\n${plan.summary.trim()}\n`, "utf8"),
            metadata: {
              event_id: event.eventId, event_type: event.eventType, step_count: plan.steps.length,
              ...(confirmation === undefined ? {} : { read_scope: "owner_confirmed" })
            },
            ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
            ...(context.traceId === undefined ? {} : { traceId: context.traceId })
          });
          artifactSpan.setAttribute("dipole.agent.artifact.size_bytes", value.sizeBytes);
          return value;
        });
        span.setAttribute("dipole.agent.run.step_count", plan.steps.length);
        return {
          kind: "complete",
          output: {
            summary: plan.summary,
            stepCount: plan.steps.length,
            ...(artifact === undefined ? {} : { artifactId: artifact.artifactId, artifactVersion: artifact.version })
          }
        };
      });
    }
  };
}

function resolveReadScopeConfirmation(
  input: AgentTaskActivityInput,
  context: ExecutionContext
): { readonly conversationId: string; readonly plan: ShadowPlan } {
  const checkpoint = readScopeCheckpointSchema.parse(input.checkpoint);
  const plan = resumedReadPlan(checkpoint);
  if (checkpoint.stepNo !== plan.steps.length) {
    throw new Error("Temporal read scope confirmation Step position mismatch");
  }
  const resume = input.resume;
  if (resume?.kind !== "input") {
    throw new Error("Temporal read scope confirmation requires the durable Elicitation response");
  }
  if (resume.requestId !== readScopeRequestId(context.taskId, context.runId, checkpoint.stepNo)) {
    throw new Error("Temporal read scope confirmation request binding mismatch");
  }
  const conversationId = resume.value[readScopeFieldId];
  if (typeof conversationId !== "string" || !checkpoint.candidates.includes(conversationId)) {
    throw new Error("Temporal read scope confirmation is outside the discovered conversations");
  }
  return { conversationId, plan };
}

// The paused plan is a validated conversation.list to conversation.read pair, so
// the resumed Run rebuilds that shape instead of re-planning: a second model plan
// could reorder Steps and break the trajectory Step numbering it must replay.
function resumedReadPlan(checkpoint: ReadScopeCheckpoint): ShadowPlan {
  return {
    summary: checkpoint.planSummary,
    steps: [
      { capabilityId: "conversation.list", input: {} },
      {
        capabilityId: "conversation.read",
        input: {
          conversationId: discoveredConversationMarker,
          ...(checkpoint.readLimit === undefined ? {} : { limit: checkpoint.readLimit })
        }
      }
    ]
  };
}

function readLimitOf(plan: ShadowPlan): { readLimit?: number } {
  const limit = plan.steps.at(-1)?.input.limit;
  return typeof limit === "number" && Number.isInteger(limit) && limit >= 1 && limit <= 100 ? { readLimit: limit } : {};
}

function readScopeRequestId(taskId: string, runId: string, stepNo: number): string {
  const canonical = [readScopeConfirmationKind, taskId, runId, String(stepNo)].join("\n");
  return `input:${createHash("sha256").update(canonical, "utf8").digest("hex").slice(0, 58)}`;
}

function readScopePrompt(offered: number, discovered: number): string {
  return discovered > offered
    ? `Select which conversation to read. ${discovered} conversations were discovered and the first ${offered} are offered.`
    : `Select which of the ${offered} discovered conversations to read.`;
}

function readScopeForm(candidates: readonly string[]): AgentElicitationForm {
  return validateElicitationForm({
    schemaVersion: agentElicitationSchemaVersion,
    fields: [{ id: readScopeFieldId, label: "Conversation to read", type: "select", required: true, options: [...candidates] }]
  });
}
