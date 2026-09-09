import { executionContextSchema } from "../runtime/execution-context.js";
import {
  agentEventSchema,
  agentRunId,
  agentTaskId,
  executeShadowPlan,
  type ShadowPlanExecutionDependencies
} from "../events/shadow-processor.js";
import type { AgentTaskActivities } from "./agent-task-activities.js";
import type { AgentArtifactCreateInput, AgentArtifactRecord } from "../capabilities/agent-capability-rpc.js";
import { AgentTelemetry } from "../observability/agent-telemetry.js";
import type { AgentRuntimeMode } from "../capabilities/agent-capability-rpc.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import type { AgentCapabilityRPCClient, AgentToolActionReference } from "../capabilities/agent-capability-rpc.js";
import { McpToolInvocationRunner } from "../mcp/mcp-tool-invocation.js";
import { createInteractiveMessageExecutor } from "../mcp/mcp-message-write-projection.js";
import { canonicalMcpJSON } from "../mcp/canonical-json.js";
import { createHash } from "node:crypto";

interface AgentArtifactWriter {
  createArtifact(input: AgentArtifactCreateInput): Promise<AgentArtifactRecord>;
}

interface AgentContextResolver {
  resolveMcpContext(taskId: string, runId: string, principalUserId: string, context?: { requestId?: string; traceId?: string }): Promise<ExecutionContext>;
}

type AgentAssistantReplyWriter = Pick<AgentCapabilityRPCClient, "begin" | "finishToolInvocation" | "executeMessageCommand">;
type AgentApprovalWriter = Pick<AgentCapabilityRPCClient, "begin" | "finishToolInvocation" | "consumeApproval" | "resolveApprovalGrant" | "executeMessageCommand">;

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
      if ((input.step !== 0 && (input.step !== 1 || input.resume?.kind !== "approval")) || input.admission === undefined || input.shadowEvent === undefined) {
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
        const systemMessage = requestedSystemMessage(event, context);
        if (systemMessage !== undefined && dependencies.approvalWriter !== undefined) {
          if (input.resume?.kind === "approval") {
            const result = await createInteractiveMessageExecutor(dependencies.approvalWriter).execute({
              conversationId: systemMessage.conversationKey,
              content: systemMessage.content
            }, context);
            return {
              kind: "complete",
              output: { summary: "Approved system message delivered", result }
            };
          }
          return {
            kind: "wait_approval",
            requestId: systemMessage.requestId,
            summary: `Send system message: ${systemMessage.content.slice(0, 160)}`,
            approval: systemMessage.approval,
            checkpoint: { kind: "system_message", content: systemMessage.content, conversationKey: systemMessage.conversationKey }
          };
        }
        const plan = await executeShadowPlan(event, context, { ...dependencies, telemetry });
        const conversationKey = activeReplyConversationKey(event, context);
        const reply = runtimeMode === "active" && conversationKey !== undefined && dependencies.replyWriter !== undefined
          ? await writeAssistantReply(dependencies.replyWriter, context, plan.summary, conversationKey)
          : undefined;
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

function requestedSystemMessage(event: ReturnType<typeof agentEventSchema.parse>, context: ExecutionContext): {
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
  if (event.eventType !== "message.direct.created") return undefined;
  const conversationKey = activeReplyConversationKey(event, context);
  const content = typeof event.payload.content === "string"
    ? event.payload.content.trim().replace(/^\/system\s+/i, "").trim()
    : "";
  if (conversationKey === undefined || content.length === 0 || !/^\/system\s+/i.test(String(event.payload.content ?? ""))) return undefined;
  const scope = { resourceType: "conversation", resourceId: conversationKey, actions: ["write"] };
  if (!context.permissions.includes("message.write") || !hasWriteScope(context, scope)) return undefined;
  const argumentsJson = canonicalMcpJSON({ conversationId: conversationKey, content });
  const material = ["dipole.agent.system-message.v1", context.taskId, context.runId, conversationKey, content].join("\n");
  const token = createHash("sha256").update(material, "utf8").digest("hex");
  return {
    content,
    conversationKey,
    requestId: `approval:${token.slice(0, 55)}`,
    approval: {
      approvalId: `approval:${token.slice(0, 55)}`,
      capabilityId: "message.system.send",
      resourceScope: scope,
      scopeSha256: sha256(["dipole.agent.scope.v1", scope.resourceType, scope.resourceId, ...scope.actions].join("\n")),
      argumentsSha256: sha256(argumentsJson),
      nonceSha256: sha256(`dipole.agent.approval-nonce.v1\n${token}`),
      expiresAtUnixMs: Date.now() + 10 * 60_000
    }
  };
}

function hasWriteScope(context: ExecutionContext, requested: { resourceType: string; resourceId: string; actions: readonly string[] }): boolean {
  return context.resourceScopes.some(scope => scope.resourceType === requested.resourceType &&
    (scope.resourceId === requested.resourceId || scope.resourceId === "*") && requested.actions.every(action => scope.actions.includes(action)));
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
