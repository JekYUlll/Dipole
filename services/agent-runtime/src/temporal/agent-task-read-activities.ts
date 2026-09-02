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
import type { AgentApprovalBinding } from "../capabilities/agent-capability-rpc.js";
import { AgentTelemetry } from "../observability/agent-telemetry.js";
import type { AgentRuntimeMode } from "../capabilities/agent-capability-rpc.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import { agentElicitationSchemaVersion, validateElicitationForm, type AgentElicitationForm } from "../task/agent-elicitation.js";
import type { AgentTaskResume } from "../task/agent-task-state.js";
import { canonicalMcpJSON } from "../mcp/canonical-json.js";

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

interface InteractiveMessageExecutor {
  execute(input: { readonly conversationId: string; readonly content: string }, context: ExecutionContext): Promise<string>;
}

interface InteractiveMessageCheckpoint {
  readonly schemaVersion: "dipole.agent.interactive-message.v1";
  readonly requestId: string;
  readonly approval: AgentApprovalBinding;
  readonly input: { readonly conversationId: string; readonly content: string };
}

export function createTemporalReadStepActivities(
  dependencies: ShadowPlanExecutionDependencies & {
    readonly artifacts?: AgentArtifactWriter;
    readonly runtimeMode?: AgentRuntimeMode;
    readonly contextResolver?: AgentContextResolver;
    readonly readPermissions?: readonly string[];
    readonly readScopeConfirmationTtlMs?: number;
    readonly interactiveMessage?: InteractiveMessageExecutor;
    readonly subscriptionMessage?: InteractiveMessageExecutor;
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
        triggerRef: event.aggregateId,
        ...(event.subscriptionId === undefined ? {} : { subscriptionId: event.subscriptionId })
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
      const resolvedContext = dependencies.contextResolver === undefined
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
      if (resolvedContext.mode !== runtimeMode || resolvedContext.taskId !== input.taskId || resolvedContext.runId !== input.runId ||
          (resolvedContext.eventId !== undefined && resolvedContext.eventId !== event.eventId)) {
        throw new Error("Core Context binding mismatch for Temporal read Step");
      }
      // Admission binds the event before this Activity starts. The Core context
      // API owns task/run authorization and intentionally does not echo eventId.
      const context = resolvedContext.eventId === event.eventId
        ? resolvedContext
        : executionContextSchema.parse({ ...resolvedContext, eventId: event.eventId });
      const requestedMessage = requestedInteractiveMessage(event);
      const interactiveMessage = requestedMessage === undefined ? undefined : {
        conversationId: directConversationKey(context.principalUuid, context.agentUuid),
        content: requestedMessage.content
      };
      if (runtimeMode === "active" && interactiveMessage !== undefined) {
        if (dependencies.interactiveMessage === undefined) {
          return { kind: "failed", message: "Interactive Agent message execution is disabled" };
        }
        return executeInteractiveMessageStep(input.taskId, input.runId, input.step, input.checkpoint, input.resume, event, context, interactiveMessage, dependencies.interactiveMessage);
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
        const replyMessageAction = await maybeSendSubscriptionReply(event, context, plan, runtimeMode, dependencies, telemetry);
        if (replyMessageAction !== undefined) span.setAttribute("dipole.agent.run.reply", "sent");
        return {
          kind: "complete",
          output: {
            summary: plan.summary,
            stepCount: plan.steps.length,
            ...(artifact === undefined ? {} : { artifactId: artifact.artifactId, artifactVersion: artifact.version }),
            ...(replyMessageAction === undefined ? {} : { replyMessageAction })
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

function executeInteractiveMessageStep(
  taskId: string,
  runId: string,
  step: number,
  checkpoint: unknown,
  resume: AgentTaskResume | undefined,
  event: { readonly eventId: string; readonly occurredAt: string },
  context: ExecutionContext,
  message: { readonly conversationId: string; readonly content: string },
  executor: InteractiveMessageExecutor
) {
  if (step === 0) {
    if (checkpoint !== undefined || resume !== undefined) throw new Error("Interactive Agent message command has an unexpected initial state");
    const approval = interactiveMessageApproval(taskId, runId, event, message);
    const requestId = `approval:${digest([taskId, runId, approval.approvalId, "request"])}`;
    return {
      kind: "wait_approval" as const,
      requestId,
      summary: "Send one system message to your direct Agent conversation",
      approval,
      checkpoint: {
        schemaVersion: "dipole.agent.interactive-message.v1" as const,
        requestId,
        approval,
        input: message
      } satisfies InteractiveMessageCheckpoint
    };
  }
  if (step !== 1) throw new Error("Interactive Agent message command exceeds its single approved execution step");
  const pending = interactiveMessageCheckpoint(checkpoint);
  if (resume?.kind !== "approval" || resume.decision !== "approved" || resume.requestId !== pending.requestId || resume.approvalId !== pending.approval.approvalId) {
    throw new Error("Interactive Agent message approval resume binding is invalid");
  }
  if (pending.input.conversationId !== message.conversationId || pending.input.content !== message.content) {
    throw new Error("Interactive Agent message checkpoint conflicts with the request");
  }
  return executor.execute(pending.input, context).then(action => ({
    kind: "complete" as const,
    output: {
      summary: "Sent one approved system message to your direct Agent conversation",
      action
    }
  }));
}

const maxSubscriptionReplyBytes = 16 * 1024;

// Autonomous replies fire only for subscription-triggered active tasks with the
// reply surface wired. They deliver the model summary to the owner's direct
// Agent conversation, reusing the audited approval-bound message write.
async function maybeSendSubscriptionReply(
  event: { readonly subscriptionId?: string | undefined },
  context: ExecutionContext,
  plan: ShadowPlan,
  runtimeMode: AgentRuntimeMode,
  dependencies: { readonly subscriptionMessage?: InteractiveMessageExecutor },
  telemetry: Pick<AgentTelemetry, "withSpan">
): Promise<string | undefined> {
  if (runtimeMode !== "active" || dependencies.subscriptionMessage === undefined || event.subscriptionId === undefined) {
    return undefined;
  }
  const content = plan.summary.trim().slice(0, maxSubscriptionReplyBytes);
  if (content.length === 0) return undefined;
  const executor = dependencies.subscriptionMessage;
  return telemetry.withSpan("agent.message.reply", {
    taskId: context.taskId, runId: context.runId,
    attributes: { "dipole.agent.message.kind": "system_message" }
  }, () => executor.execute(
    { conversationId: directConversationKey(context.principalUuid, context.agentUuid), content },
    context
  ));
}

function requestedInteractiveMessage(event: { readonly eventType: string; readonly payload: Record<string, unknown> }): { readonly content: string } | undefined {
  if (event.eventType !== "agent.interactive.requested" || typeof event.payload.content !== "string") return undefined;
  const content = event.payload.content.trim();
  if (!content.startsWith("/send ")) return undefined;
  const message = content.slice("/send ".length).trim();
  if (!message) throw new Error("Interactive Agent message content is required");
  return { content: message };
}

function directConversationKey(first: string, second: string): string {
  return `direct:${[first.trim(), second.trim()].sort().join(":")}`;
}

function interactiveMessageApproval(
  taskId: string,
  runId: string,
  event: { readonly eventId: string; readonly occurredAt: string },
  requested: { readonly conversationId: string; readonly content: string }
): AgentApprovalBinding {
  const occurredAt = Date.parse(event.occurredAt);
  if (!Number.isSafeInteger(occurredAt)) throw new Error("Interactive Agent message event time is invalid");
  const conversationId = requested.conversationId;
  if (!conversationId) throw new Error("Interactive Agent message conversation is unavailable");
  const resourceScope = { resourceType: "conversation", resourceId: conversationId, actions: ["write"] };
  const argumentsSha256 = digest([canonicalMcpJSON(requested)]);
  const scopeSha256 = digest(["dipole.agent.scope.v1", resourceScope.resourceType, resourceScope.resourceId, ...resourceScope.actions]);
  // Agent approvals use a VARCHAR(64) primary key. Keep a deterministic prefix
  // and reserve a bounded digest for their idempotent write key.
  const approvalId = `approval:${digest(["dipole.agent.interactive-message.v1", taskId, runId, event.eventId, argumentsSha256]).slice(0, 48)}`;
  return {
    approvalId,
    capabilityId: "message.system.send",
    resourceScope,
    scopeSha256,
    argumentsSha256,
    nonceSha256: digest(["dipole.agent.interactive-message.nonce.v1", approvalId]),
    expiresAtUnixMs: occurredAt + 30 * 60 * 1_000
  };
}

function interactiveMessageCheckpoint(value: unknown): InteractiveMessageCheckpoint {
  if (!isRecord(value) || value.schemaVersion !== "dipole.agent.interactive-message.v1" || typeof value.requestId !== "string" ||
      !isRecord(value.approval) || !isRecord(value.input) || typeof value.input.conversationId !== "string" || typeof value.input.content !== "string") {
    throw new Error("Interactive Agent message checkpoint is invalid");
  }
  const approval = value.approval;
  if (typeof approval.approvalId !== "string" || typeof approval.capabilityId !== "string" || !isRecord(approval.resourceScope) ||
      typeof approval.resourceScope.resourceType !== "string" || typeof approval.resourceScope.resourceId !== "string" || !Array.isArray(approval.resourceScope.actions) ||
      approval.resourceScope.actions.some(action => typeof action !== "string") || typeof approval.scopeSha256 !== "string" ||
      typeof approval.argumentsSha256 !== "string" || typeof approval.nonceSha256 !== "string" || typeof approval.expiresAtUnixMs !== "number") {
    throw new Error("Interactive Agent message Approval checkpoint is invalid");
  }
  return value as unknown as InteractiveMessageCheckpoint;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function digest(parts: readonly string[]): string {
  return createHash("sha256").update(parts.join("\n"), "utf8").digest("hex");
}
