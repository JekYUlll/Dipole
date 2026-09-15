import { executionContextSchema } from "../runtime/execution-context.js";
import {
  agentEventSchema,
  agentRunId,
  agentTaskId,
  executeShadowPlan,
  type ShadowPlanExecutionDependencies
} from "../events/shadow-processor.js";
import type { AgentTaskActivities, AgentTaskActivityInput, AgentTaskDirective } from "./agent-task-activities.js";
import type { AgentArtifactCreateInput, AgentArtifactRecord } from "../capabilities/agent-capability-rpc.js";
import { AgentTelemetry } from "../observability/agent-telemetry.js";
import type { AgentRuntimeMode } from "../capabilities/agent-capability-rpc.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import type { AgentCapabilityRPCClient, AgentToolActionReference } from "../capabilities/agent-capability-rpc.js";
import { McpToolInvocationRunner } from "../mcp/mcp-tool-invocation.js";
import { createInteractiveMessageExecutor } from "../mcp/mcp-message-write-projection.js";
import { canonicalMcpJSON } from "../mcp/canonical-json.js";
import { createHash } from "node:crypto";
import { z } from "zod";

const messageCheckpointSchema = z.object({
  kind: z.literal("system_message"), content: z.string().trim().min(1).max(2000), conversationKey: z.string().min(1),
  publishAtUnixMs: z.number().int().positive().optional()
}).strict();

interface AgentArtifactWriter {
  createArtifact(input: AgentArtifactCreateInput): Promise<AgentArtifactRecord>;
}

interface AgentContextResolver {
  resolveMcpContext(taskId: string, runId: string, principalUserId: string, context?: { requestId?: string; traceId?: string }): Promise<ExecutionContext>;
}

type AgentAssistantReplyWriter = Pick<AgentCapabilityRPCClient, "begin" | "finishToolInvocation" | "executeMessageCommand">;
type AgentApprovalWriter = Pick<AgentCapabilityRPCClient, "beginMcpToolCommand" | "finishToolInvocation" | "consumeApproval" | "resolveApprovalGrant" | "executeMessageCommand">;

export function createTemporalReadStepActivities(
  dependencies: ShadowPlanExecutionDependencies & {
    readonly artifacts?: AgentArtifactWriter;
    readonly runtimeMode?: AgentRuntimeMode;
    readonly contextResolver?: AgentContextResolver;
    readonly replyWriter?: AgentAssistantReplyWriter;
    readonly approvalWriter?: AgentApprovalWriter;
  }
): AgentTaskActivities {
  return {
    async executeAgentTaskStep(input) {
      if (input.admission === undefined || input.shadowEvent === undefined) {
        throw new Error("Temporal read Step requires an initial step or an approval resume with trusted admission and shadow event");
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
          permissions: ["conversation.list", "conversation.read"],
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
	  if (context.mode !== runtimeMode || context.taskId !== input.taskId || context.runId !== input.runId) {
		throw new Error("Core Context binding mismatch for Temporal read Step");
	  }
      const telemetry = dependencies.telemetry ?? new AgentTelemetry();
      return telemetry.withSpan("agent.run", {
        taskId: context.taskId, runId: context.runId,
        attributes: { "dipole.agent.mode": context.mode, "dipole.agent.event.type": event.eventType }
      }, async span => {
        const report = collaborationReport(event);
        if (report !== undefined) return executeCollaborationReport(input, event, context, dependencies, report);
        if (input.step !== 0 && (input.step !== 1 || input.resume?.kind !== "approval")) {
          throw new Error("Unexpected message task resume");
        }
        const restored = input.resume?.kind === "approval" && input.checkpoint !== undefined
          ? messageCheckpointSchema.parse(input.checkpoint) : undefined;
        if (restored !== undefined && restored.conversationKey !== activeReplyConversationKey(event, context)) {
          throw new Error("Approval checkpoint conversation mismatch");
        }
        const schedule = scheduledDigest(event);
        if (schedule !== undefined && input.resume === undefined && schedule.publishAtUnixMs <= Date.now()) {
          throw new Error("Digest publication time has already passed");
        }
        if (restored?.publishAtUnixMs !== undefined && restored.publishAtUnixMs !== schedule?.publishAtUnixMs) {
          throw new Error("Approval checkpoint publication time mismatch");
        }
        const explicitMessage = requestedSystemMessage(event, context, restored?.content, restored?.publishAtUnixMs);
        // Approval resumes use the exact checkpoint, without another model call.
        const plan = explicitMessage === undefined && input.resume === undefined
          ? await executeShadowPlan(schedule === undefined ? event : {
            ...event, payload: { ...event.payload, content: schedule.query }
          }, context, { ...dependencies, telemetry }) : undefined;
        const systemMessage = explicitMessage ?? (schedule !== undefined && plan !== undefined
          ? requestedSystemMessage(event, context, plan.summary, schedule.publishAtUnixMs)
          : plan?.proposedWrite === undefined ? undefined : requestedSystemMessage(event, context, plan.proposedWrite.content));
        if ((input.resume !== undefined || plan?.proposedWrite !== undefined || schedule !== undefined) && systemMessage === undefined) {
          throw new Error("Message proposal is outside authorized scope");
        }
        if (systemMessage !== undefined && dependencies.approvalWriter !== undefined) {
          if (input.resume?.kind === "approval") {
            if (schedule !== undefined && Date.now() < schedule.publishAtUnixMs) {
              throw new Error("Digest publication time has not arrived");
            }
            if (input.resume.decision !== "approved" || input.resume.requestId !== systemMessage.requestId ||
                input.resume.approvalId !== systemMessage.approval.approvalId) {
              throw new Error("Approval resume binding mismatch");
            }
            const result = await createInteractiveMessageExecutor(dependencies.approvalWriter).execute({
              conversationId: systemMessage.conversationKey,
              content: systemMessage.content
            }, context, input.resume.approvalId);
            return {
              kind: "complete",
              output: { summary: "Approved system message delivered", result }
            };
          }
          return {
            kind: "wait_approval",
            requestId: systemMessage.requestId,
            summary: `${schedule === undefined ? "Send system message" : `Publish at ${new Date(schedule.publishAtUnixMs).toISOString()} to ${systemMessage.conversationKey}`}: ${systemMessage.content}`,
            approval: systemMessage.approval,
            ...(schedule === undefined ? {} : { notBeforeUnixMs: schedule.publishAtUnixMs }),
            checkpoint: { kind: "system_message", content: systemMessage.content, conversationKey: systemMessage.conversationKey,
              ...(schedule === undefined ? {} : { publishAtUnixMs: schedule.publishAtUnixMs }) }
          };
        }
        if (systemMessage !== undefined || plan === undefined) throw new Error("Message approval writer is unavailable");
        const conversationKey = activeReplyConversationKey(event, context);
        const reply = runtimeMode === "active" && conversationKey !== undefined && dependencies.replyWriter !== undefined
          ? await writeAssistantReply(dependencies.replyWriter, context, plan.summary, conversationKey)
          : undefined;
        const artifact = dependencies.artifacts === undefined || runtimeMode === "active" ? undefined : await telemetry.withSpan("agent.artifact.create", {
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
            metadata: { event_id: event.eventId, event_type: event.eventType, step_count: plan.steps.length },
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
            ...(reply === undefined ? {} : { replyMessageId: reply.resourceId }),
            ...(artifact === undefined ? {} : { artifactId: artifact.artifactId, artifactVersion: artifact.version })
          }
        };
      });
    }
  };
}

function requestedSystemMessage(event: ReturnType<typeof agentEventSchema.parse>, context: ExecutionContext, proposedContent?: string, publishAtUnixMs?: number): {
  content: string;
  conversationKey: string;
  requestId: string;
  approval: {
    approvalId: string;
    capabilityId: string;
    resourceScope: { resourceType: string; resourceId: string; actions: readonly string[] };
    scopeSha256: string;
    argumentsSha256: string;
    nonceSha256: string;
    expiresAtUnixMs: number;
  };
} | undefined {
  if (event.eventType !== "message.direct.created" && !(event.eventType === "message.group.created" && (publishAtUnixMs !== undefined || collaborationReport(event) !== undefined))) return undefined;
  const conversationKey = activeReplyConversationKey(event, context);
  const content = proposedContent ?? (typeof event.payload.content === "string"
    ? event.payload.content.trim().replace(/^\/system\s+/i, "").trim()
    : "");
  if (context.mode !== "active" || conversationKey === undefined || content.length === 0 || content.length > 2000 ||
      (proposedContent === undefined && !/^\/system\s+/i.test(String(event.payload.content ?? "")))) return undefined;
  const scope = { resourceType: "conversation", resourceId: conversationKey, actions: ["write"] };
  if (!context.permissions.includes("message.write") || !hasWriteScope(context, scope)) return undefined;
  const argumentsJson = canonicalMcpJSON({ conversationId: conversationKey, content });
  const material = ["dipole.agent.system-message.v1", context.taskId, context.runId, conversationKey, content,
    ...(publishAtUnixMs === undefined ? [] : [String(publishAtUnixMs)])].join("\n");
  const token = createHash("sha256").update(material, "utf8").digest("hex");
  return {
    content,
    conversationKey,
    requestId: `approval:${token.slice(0, 55)}`,
    approval: {
      approvalId: `approval:${token.slice(0, 55)}`,
      capabilityId: conversationKey.startsWith("group:") ? "message.group_reply.send" : "message.system.send",
      resourceScope: scope,
      scopeSha256: sha256(["dipole.agent.scope.v1", scope.resourceType, scope.resourceId, ...scope.actions].join("\n")),
      argumentsSha256: sha256(argumentsJson),
      nonceSha256: sha256(`dipole.agent.approval-nonce.v1\n${token}`),
      expiresAtUnixMs: publishAtUnixMs === undefined ? Date.now() + 10 * 60_000 : publishAtUnixMs + 10 * 60_000
    }
  };
}

function scheduledDigest(event: ReturnType<typeof agentEventSchema.parse>): { publishAtUnixMs: number; query: string } | undefined {
  const content = String(event.payload.content ?? "").trim().replace(/^@(?:Dipole\s+AI|AI)\s+/i, "");
  if (!/^\/digest(?:\s|$)/i.test(content)) return undefined;
  const match = /^\/digest\s+(\S+)\s+([\s\S]+)$/i.exec(content);
  if (!["message.direct.created", "message.group.created"].includes(event.eventType) || match === null || !/(?:Z|[+-]\d{2}:\d{2})$/.test(match[1]!)) {
    throw new Error("Use /digest <ISO timestamp with timezone> <retrieval request>");
  }
  const publishAtUnixMs = Date.parse(match[1]!);
  const occurredAt = Date.parse(event.occurredAt);
  if (!Number.isSafeInteger(publishAtUnixMs) || publishAtUnixMs <= occurredAt || publishAtUnixMs > occurredAt + 7 * 86400_000) {
    throw new Error("Digest publication must be within seven days after the request");
  }
  return { publishAtUnixMs, query: match[2]!.trim() };
}

function hasWriteScope(context: ExecutionContext, requested: { resourceType: string; resourceId: string; actions: readonly string[] }): boolean {
  return context.resourceScopes.some(scope => scope.resourceType === requested.resourceType &&
    (scope.resourceId === requested.resourceId || scope.resourceId === "*") && requested.actions.every(action => scope.actions.includes(action)));
}

const reportCheckpointSchema = z.object({
  kind: z.literal("collaboration_report"), phase: z.enum(["question", "draft", "approval"]),
  summary: z.string().min(1).max(2000), conversationKey: z.string().min(1),
  deadline: z.number().int().positive(), requestId: z.string().min(1)
}).strict();

function collaborationReport(event: ReturnType<typeof agentEventSchema.parse>): { deadline: number; query: string } | undefined {
  const content = String(event.payload.content ?? "").trim().replace(/^@(?:Dipole\s+AI|AI)\s+/i, "");
  if (!/^\/report(?:\s|$)/i.test(content)) return undefined;
  const match = /^\/report\s+(\S+)\s+([\s\S]+)$/i.exec(content);
  const deadline = match === null ? NaN : Date.parse(match[1]!);
  const occurred = Date.parse(event.occurredAt);
  if (match === null || !/(?:Z|[+-]\d{2}:\d{2})$/.test(match[1]!) || !Number.isSafeInteger(deadline) ||
      deadline <= occurred || deadline > occurred + 7 * 86400_000 || match[2]!.length > 500) {
    throw new Error("Use /report <ISO deadline with timezone within seven days> <request up to 500 characters>");
  }
  return { deadline, query: match[2]!.trim() };
}

async function executeCollaborationReport(
  input: AgentTaskActivityInput, event: ReturnType<typeof agentEventSchema.parse>, context: ExecutionContext,
  dependencies: Parameters<typeof createTemporalReadStepActivities>[0], report: { deadline: number; query: string }
): Promise<AgentTaskDirective> {
  const conversationKey = activeReplyConversationKey(event, context);
  if (context.mode !== "active" || !conversationKey || !dependencies.approvalWriter ||
      !dependencies.planner.reviewReport || !dependencies.planner.finishReport || !dependencies.artifacts) {
    throw new Error("Collaboration report requires active model, artifacts and approved message execution");
  }
  const requestEvent = { ...event, payload: { ...event.payload, content: `Read this conversation and retrieve evidence for: ${report.query}. Prepare a factual report; do not propose a write.` } };
  let summary: string;
  if (input.step === 0 && input.resume === undefined) {
    const plan = await executeShadowPlan(requestEvent, context, dependencies);
    summary = plan.summary;
    const review = await dependencies.planner.reviewReport(requestEvent, context, summary);
    if (review.question && Date.now() < report.deadline) {
      const requestId = `report:${sha256(`${context.taskId}:question`).slice(0, 55)}`;
      return {
        kind: "wait_input", requestId, prompt: review.question, expiresAtUnixMs: report.deadline,
        form: { schemaVersion: "dipole.agent.elicitation.v1", fields: [{ id: "answer", label: "补充信息（留空表示暂时未知）", type: "text", required: false, maxLength: 1500 }] },
        timeoutValue: { answer: "" },
        checkpoint: { kind: "collaboration_report", phase: "question", summary, conversationKey, deadline: report.deadline, requestId }
      };
    }
    summary = await dependencies.planner.finishReport(requestEvent, context, summary, "No owner input. Mark any missing information unknown.");
  } else {
    const saved = reportCheckpointSchema.parse(input.checkpoint);
    if (saved.conversationKey !== conversationKey || saved.deadline !== report.deadline || input.resume?.requestId !== saved.requestId) {
      throw new Error("Report checkpoint binding mismatch");
    }
    if (saved.phase === "approval" && input.resume.kind === "approval") {
      const proposed = requestedSystemMessage(event, context, saved.summary);
      if (!proposed || input.resume.approvalId !== proposed.approval.approvalId || input.resume.decision !== "approved") {
        throw new Error("Report approval binding mismatch");
      }
      const result = await createInteractiveMessageExecutor(dependencies.approvalWriter).execute(
        { conversationId: conversationKey, content: saved.summary }, context, input.resume.approvalId);
      return { kind: "complete", output: { summary: saved.summary, result } };
    }
    if (input.resume.kind !== "input") throw new Error("Report requires task-bound owner input");
    if (saved.phase === "question") {
      const answer = z.object({ answer: z.string().max(1500).optional() }).strict().parse(input.resume.value);
      summary = await dependencies.planner.finishReport(requestEvent, context, saved.summary,
        answer.answer?.trim() || "Owner did not provide information before continuing. Mark missing facts unknown.");
    } else if (saved.phase === "draft") {
      const edit = z.object({ content: z.string().trim().max(1800).optional() }).strict().parse(input.resume.value);
      summary = edit.content || saved.summary;
      const proposed = requestedSystemMessage(event, context, summary);
      if (!proposed) throw new Error("Report destination is outside authorized scope");
      await dependencies.artifacts.createArtifact({ tenantId: context.tenantId, taskId: context.taskId, runId: context.runId,
        artifactType: "conversation_digest", version: 2, title: "Reviewed collaboration report", mediaType: "text/markdown",
        content: Buffer.from(summary), metadata: { conversation_key: conversationKey } });
      return { kind: "wait_approval", requestId: proposed.requestId, summary: `发布到 ${conversationKey}\n\n${summary}`,
        approval: proposed.approval,
        checkpoint: { ...saved, phase: "approval", summary, requestId: proposed.requestId } };
    } else throw new Error("Unexpected report input phase");
  }
  const requestId = `report:${sha256(`${context.taskId}:draft`).slice(0, 55)}`;
  await dependencies.artifacts.createArtifact({ tenantId: context.tenantId, taskId: context.taskId, runId: context.runId,
    artifactType: "conversation_digest", version: 1, title: "Collaboration report draft", mediaType: "text/markdown",
    content: Buffer.from(summary), metadata: { conversation_key: conversationKey } });
  return { kind: "wait_input", requestId, prompt: `草稿预览（留空保留原文，填写内容可替换草稿）\n\n${summary}`,
    expiresAtUnixMs: Math.max(report.deadline, Date.now()) + 86400_000,
    form: { schemaVersion: "dipole.agent.elicitation.v1", fields: [{ id: "content", label: "修改后的完整草稿", type: "text", required: false, maxLength: 1800 }] },
    checkpoint: { kind: "collaboration_report", phase: "draft", summary, conversationKey, deadline: report.deadline, requestId } };
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}

function activeReplyConversationKey(event: ReturnType<typeof agentEventSchema.parse>, context: ExecutionContext): string | undefined {
  const conversationKey = typeof event.payload.conversation_key === "string" ? event.payload.conversation_key.trim() : "";
  if (event.eventType === "message.direct.created") {
    const expected = `direct:${[context.principalUuid, context.agentUuid].sort().join(":")}`;
    return conversationKey === expected ? conversationKey : undefined;
  }
  if (event.eventType === "message.group.created" && /^group:[^:\s]+$/.test(conversationKey)) {
    return conversationKey;
  }
  return undefined;
}

async function writeAssistantReply(writer: AgentAssistantReplyWriter, context: ExecutionContext, content: string, conversationKey: string): Promise<AgentToolActionReference> {
  const normalized = content.trim();
  if (!normalized) throw new Error("Agent reply content is empty");
  const groupReply = conversationKey.startsWith("group:");
  const capabilityId = groupReply ? "message.group_reply.send" : "message.assistant_reply.send";
  const commandKind = groupReply ? "group_reply" : "assistant_reply";
  const invocationId = `tool:${createHash("sha256").update([
    "dipole.agent.assistant-reply.v1", context.taskId, context.runId, capabilityId, conversationKey, normalized
  ].join("\n"), "utf8").digest("hex").slice(0, 59)}`;
  const runner = new McpToolInvocationRunner(
    { begin: input => writer.begin(input), finish: input => writer.finishToolInvocation(input) },
    undefined,
    () => invocationId
  );
  let reference: AgentToolActionReference | undefined;
  await runner.execute(
    { name: "dipole_assistant_reply", capabilityId },
    { content: normalized, conversationId: conversationKey },
    context,
    async (signal, toolInvocationId) => {
      if (signal.aborted) throw new Error("Agent reply was cancelled");
      reference = await writer.executeMessageCommand({
        taskId: context.taskId, runId: context.runId, invocationId: toolInvocationId,
        commandKind, content: normalized, conversationKey,
        ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
        ...(context.traceId === undefined ? {} : { traceId: context.traceId })
      });
      return reference;
    },
    result => result as AgentToolActionReference
  );
  if (reference === undefined) throw new Error("Agent reply did not return an action reference");
  return reference;
}
