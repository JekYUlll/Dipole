import { createHash } from "node:crypto";
import { z } from "zod";

import type { AgentApprovalBinding, AgentCapabilityRPCClient, AgentToolActionReference } from "../capabilities/agent-capability-rpc.js";
import { CapabilityRegistry } from "../capabilities/registry.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import { canonicalMcpJSON } from "./canonical-json.js";
import type { DipoleMcpWriteExecutor, DipoleMcpWriteToolProjection } from "./dipole-mcp-server.js";
import { McpToolInvocationRunner } from "./mcp-tool-invocation.js";
import {
  createMcpWriteApprovalConsumePort,
  createMcpWriteApprovalGrantResolver,
  McpWriteApprovalGate
} from "./mcp-write-approval-gate.js";

export interface McpMessageCommandPort {
  executeMessageCommand(input: {
    readonly taskId: string;
    readonly runId: string;
    readonly invocationId: string;
    readonly commandKind: "assistant_reply" | "system_message" | "group_reply";
    readonly content: string;
    readonly requestId?: string;
    readonly traceId?: string;
  }): Promise<AgentToolActionReference>;
}

const messageInputSchema = z.object({
  conversationId: z.string().trim().min(1).max(256),
  content: z.string().trim().min(1).max(16 * 1024)
}).strict();

export class McpMessageWriteProjection implements DipoleMcpWriteExecutor {
  constructor(
    private readonly approvals: McpWriteApprovalGate,
    private readonly runner: McpToolInvocationRunner,
    private readonly commands: McpMessageCommandPort
  ) {}

  async execute(tool: DipoleMcpWriteToolProjection, rawArguments: unknown, context: ExecutionContext): Promise<string> {
    const input = messageInputSchema.parse(rawArguments);
    // Route B/B2: a group_reply tool targets the group conversation that
    // mentioned the Agent (group:<uuid>); every other message tool stays
    // limited to the owner's direct Agent conversation.
    if (tool.capabilityId === "message.group_reply.send") {
      if (!input.conversationId.startsWith("group:") || input.conversationId === "group:") {
        throw new Error("Group reply Tool requires a group conversation id");
      }
    } else if (input.conversationId !== directConversationKey(context.principalUuid, context.agentUuid)) {
      throw new Error("MCP Message Tool is limited to its authenticated direct conversation");
    }
    const approved = await this.approvals.authorize(tool.capabilityId, input, context);
    return this.runner.execute(
      { name: tool.name, capabilityId: approved.capabilityId, approvalId: approved.approvalId },
      approved.input,
      context,
      (signal, invocationId) => {
        if (signal.aborted) throw new Error("MCP Message Command was cancelled");
        return this.commands.executeMessageCommand({
          taskId: context.taskId,
          runId: context.runId,
          invocationId,
          commandKind: tool.commandKind,
          content: input.content,
          ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
          ...(context.traceId === undefined ? {} : { traceId: context.traceId })
        });
      },
      result => messageActionReference(result, tool.commandKind)
    );
  }
}

const interactiveMessageTool: DipoleMcpWriteToolProjection = {
  name: "dipole_message_send",
  capabilityId: "message.system.send",
  title: "Send message",
  description: "Send one approved system message to the task owner's direct Agent conversation",
  inputSchema: messageInputSchema,
  commandKind: "system_message"
};

const assistantReplyTool: DipoleMcpWriteToolProjection = {
  name: "dipole_assistant_reply",
  capabilityId: "message.assistant_reply.send",
  title: "Send assistant reply",
  description: "Deliver one approved assistant reply to the task owner's direct Agent conversation",
  inputSchema: messageInputSchema,
  commandKind: "assistant_reply"
};

// Route B/B2: a group @-mention reply targets the group conversation that
// triggered the task. The runtime scopes the write to group:<uuid> and Core
// re-derives the arguments digest from that conversation id.
const groupReplyTool: DipoleMcpWriteToolProjection = {
  name: "dipole_group_reply",
  capabilityId: "message.group_reply.send",
  title: "Send group reply",
  description: "Deliver one approved assistant reply to the group conversation that mentioned the Agent",
  inputSchema: messageInputSchema,
  commandKind: "group_reply"
};

export interface MessageExecutor {
  execute(input: { readonly conversationId: string; readonly content: string }, context: ExecutionContext): Promise<string>;
}

export interface SubscriptionMessageExecutor {
  execute(
    input: { readonly conversationId: string; readonly content: string; readonly eventId: string; readonly occurredAtUnixMs: number },
    context: ExecutionContext
  ): Promise<string>;
}

type MessageExecutorClient = Pick<AgentCapabilityRPCClient, "begin" | "finishToolInvocation" | "consumeApproval" | "resolveApprovalGrant" | "executeMessageCommand">;
type SubscriptionMessageExecutorClient = MessageExecutorClient & Pick<AgentCapabilityRPCClient, "authorizeSubscriptionMessage" | "resolveMcpToolCommand">;

// Marks an autonomous reply the Runtime recognised as already delivered by a
// prior Activity attempt. The value is informational (task output only); the
// audited message and its Tool Invocation already exist in Core.
export const subscriptionReplyReplayMarker = "dipole.agent.subscription-reply.already-delivered.v1";

const interactiveMessageInvocationNamespace = "dipole.agent.interactive-message-invocation.v1";
const subscriptionMessageInvocationNamespace = "dipole.agent.subscription-message-invocation.v1";
const interactiveReplyInvocationNamespace = "dipole.agent.interactive-reply-invocation.v1";

// The owner Signal drives interactive writes.
export function createInteractiveMessageExecutor(client: MessageExecutorClient): MessageExecutor {
  const projection = messageProjectionExecutor(client, interactiveMessageInvocationNamespace);
  return { execute: (input, context) => projection(input, context) };
}

// Autonomous subscription replies have no owner Signal, so the executor first
// asks Core to mint an already-approved, subscription-scoped write grant, then
// runs the identical approval-bound write chain under a distinct Tool
// Invocation namespace so the two paths never collide inside one task.
export function createSubscriptionMessageExecutor(client: SubscriptionMessageExecutorClient): SubscriptionMessageExecutor {
  const projection = messageProjectionExecutor(client, subscriptionMessageInvocationNamespace);
  return {
    execute: async (input, context) => {
      const request = { conversationId: input.conversationId, content: input.content };
      const binding = subscriptionMessageApproval(context, input);
      try {
        await client.authorizeSubscriptionMessage(context.taskId, context.runId, binding, context);
        return await projection(request, context);
      } catch (error) {
        // A retried Activity can re-enter after a prior attempt already delivered
        // this reply and consumed its single-use write grant. The mint stays
        // idempotent, but resolve/consume then fails on the spent grant. The
        // reply write commits before that grant is spent, so a completed Tool
        // Invocation under the deterministic id is proof of delivery: recognise
        // it and skip instead of double-sending. Any other failure (no completed
        // invocation) is genuine and must surface so the Task fails loudly.
        if (await subscriptionReplyAlreadyDelivered(client, context, request)) {
          return subscriptionReplyReplayMarker;
        }
        throw error;
      }
    }
  };
}

export interface InteractiveReplyExecutor {
  execute(
    input: { readonly conversationId: string; readonly content: string; readonly eventId: string; readonly occurredAtUnixMs: number },
    context: ExecutionContext
  ): Promise<string>;
}

type InteractiveReplyExecutorClient = MessageExecutorClient & Pick<AgentCapabilityRPCClient, "authorizeInteractiveReply" | "resolveMcpToolCommand">;

// Marks an interactive reply the Runtime recognised as already delivered by a
// prior Activity attempt. Informational (task output only).
export const interactiveReplyReplayMarker = "dipole.agent.interactive-reply.already-delivered.v1";

// A self-initiated interactive task replies to its own owner. Like the
// subscription path it carries no owner Signal, so the executor first asks Core
// to mint an already-approved, owner-scoped assistant_reply grant, then runs the
// identical approval-bound write chain under its own Tool Invocation namespace so
// interactive /send writes and autonomous replies never collide inside one task.
export function createInteractiveReplyExecutor(client: InteractiveReplyExecutorClient): InteractiveReplyExecutor {
  const projection = messageProjectionExecutor(client, interactiveReplyInvocationNamespace, assistantReplyTool);
  return {
    execute: async (input, context) => {
      const request = { conversationId: input.conversationId, content: input.content };
      const binding = interactiveReplyApproval(context, input);
      try {
        await client.authorizeInteractiveReply(context.taskId, context.runId, binding, context);
        return await projection(request, context);
      } catch (error) {
        // A retried Activity can re-enter after a prior attempt already delivered
        // this reply and spent its single-use grant. A completed Tool Invocation
        // under the deterministic id is proof of delivery: skip instead of
        // double-sending; any other failure must surface so the Task fails loudly.
        if (await interactiveReplyAlreadyDelivered(client, context, request)) {
          return interactiveReplyReplayMarker;
        }
        throw error;
      }
    }
  };
}

async function interactiveReplyAlreadyDelivered(
  client: Pick<InteractiveReplyExecutorClient, "resolveMcpToolCommand">,
  context: ExecutionContext,
  request: { readonly conversationId: string; readonly content: string }
): Promise<boolean> {
  const invocationId = messageInvocationID(interactiveReplyInvocationNamespace, assistantReplyTool.capabilityId, context, request);
  try {
    const invocation = await client.resolveMcpToolCommand(context.taskId, context.runId, invocationId);
    return invocation.status === "completed";
  } catch {
    return false;
  }
}

const groupReplyInvocationNamespace = "dipole.agent.group-reply-invocation.v1";

export interface GroupReplyExecutor {
  execute(
    input: { readonly conversationId: string; readonly content: string; readonly eventId: string; readonly occurredAtUnixMs: number },
    context: ExecutionContext
  ): Promise<string>;
}

type GroupReplyExecutorClient = MessageExecutorClient & Pick<AgentCapabilityRPCClient, "authorizeGroupReply" | "resolveMcpToolCommand">;

// Marks a group reply the Runtime recognised as already delivered by a prior
// Activity attempt. Informational (task output only).
export const groupReplyReplayMarker = "dipole.agent.group-reply.already-delivered.v1";

// Route B/B2: a group @-mention interactive task replies in the group
// conversation that mentioned the Agent. Like the 1v1 interactive reply it
// carries no owner Signal, so the executor first asks Core to mint an
// already-approved, group-scoped group_reply grant (AuthorizeGroupReply), then
// runs the approval-bound write chain under a distinct Tool Invocation namespace
// so group replies never collide with 1v1 replies inside one task.
export function createGroupReplyExecutor(client: GroupReplyExecutorClient): GroupReplyExecutor {
  const projection = messageProjectionExecutor(client, groupReplyInvocationNamespace, groupReplyTool);
  return {
    execute: async (input, context) => {
      const request = { conversationId: input.conversationId, content: input.content };
      const binding = groupReplyApproval(context, input);
      try {
        await client.authorizeGroupReply(context.taskId, context.runId, binding, context);
        return await projection(request, context);
      } catch (error) {
        if (await groupReplyAlreadyDelivered(client, context, request)) {
          return groupReplyReplayMarker;
        }
        throw error;
      }
    }
  };
}

async function groupReplyAlreadyDelivered(
  client: Pick<GroupReplyExecutorClient, "resolveMcpToolCommand">,
  context: ExecutionContext,
  request: { readonly conversationId: string; readonly content: string }
): Promise<boolean> {
  const invocationId = messageInvocationID(groupReplyInvocationNamespace, groupReplyTool.capabilityId, context, request);
  try {
    const invocation = await client.resolveMcpToolCommand(context.taskId, context.runId, invocationId);
    return invocation.status === "completed";
  } catch {
    return false;
  }
}

// Deterministic so a retried group reply mints an identical binding, mirroring
// the interactive-reply derivation but scoped to the group-reply namespace and
// the group_reply capability. The scope targets the group conversation.
function groupReplyApproval(
  context: ExecutionContext,
  input: { readonly conversationId: string; readonly content: string; readonly eventId: string; readonly occurredAtUnixMs: number }
): AgentApprovalBinding {
  if (!Number.isSafeInteger(input.occurredAtUnixMs)) throw new Error("Group Agent reply event time is invalid");
  if (!input.conversationId.trim() || !input.eventId.trim()) throw new Error("Group Agent reply binding is incomplete");
  if (!input.conversationId.startsWith("group:")) throw new Error("Group Agent reply scope must target a group conversation");
  const resourceScope = { resourceType: "conversation", resourceId: input.conversationId, actions: ["write"] as const };
  const argumentsSha256 = messageDigest([canonicalMcpJSON({ conversationId: input.conversationId, content: input.content })]);
  const scopeSha256 = messageDigest(["dipole.agent.scope.v1", resourceScope.resourceType, resourceScope.resourceId, ...resourceScope.actions]);
  const approvalId = `approval:${messageDigest(["dipole.agent.group-reply.v1", context.taskId, context.runId, input.eventId, argumentsSha256]).slice(0, 48)}`;
  return {
    approvalId,
    capabilityId: "message.group_reply.send",
    resourceScope,
    scopeSha256,
    argumentsSha256,
    nonceSha256: messageDigest(["dipole.agent.group-reply.nonce.v1", approvalId]),
    expiresAtUnixMs: input.occurredAtUnixMs + 30 * 60 * 1_000
  };
}

// Deterministic so a retried reply mints an identical binding, mirroring the
// subscription approval derivation but scoped to the interactive-reply namespace
// and the assistant_reply capability.
function interactiveReplyApproval(
  context: ExecutionContext,
  input: { readonly conversationId: string; readonly content: string; readonly eventId: string; readonly occurredAtUnixMs: number }
): AgentApprovalBinding {
  if (!Number.isSafeInteger(input.occurredAtUnixMs)) throw new Error("Interactive Agent reply event time is invalid");
  if (!input.conversationId.trim() || !input.eventId.trim()) throw new Error("Interactive Agent reply binding is incomplete");
  const resourceScope = { resourceType: "conversation", resourceId: input.conversationId, actions: ["write"] };
  const argumentsSha256 = messageDigest([canonicalMcpJSON({ conversationId: input.conversationId, content: input.content })]);
  const scopeSha256 = messageDigest(["dipole.agent.scope.v1", resourceScope.resourceType, resourceScope.resourceId, ...resourceScope.actions]);
  const approvalId = `approval:${messageDigest(["dipole.agent.interactive-reply.v1", context.taskId, context.runId, input.eventId, argumentsSha256]).slice(0, 48)}`;
  return {
    approvalId,
    capabilityId: "message.assistant_reply.send",
    resourceScope,
    scopeSha256,
    argumentsSha256,
    nonceSha256: messageDigest(["dipole.agent.interactive-reply.nonce.v1", approvalId]),
    expiresAtUnixMs: input.occurredAtUnixMs + 30 * 60 * 1_000
  };
}

// Probes the deterministic reply Tool Invocation. Core's ResolveMcpToolCommand
// is a read: a missing invocation (first attempt) or a still-open one is not
// proof of delivery, so only a completed status short-circuits the replay.
async function subscriptionReplyAlreadyDelivered(
  client: Pick<SubscriptionMessageExecutorClient, "resolveMcpToolCommand">,
  context: ExecutionContext,
  request: { readonly conversationId: string; readonly content: string }
): Promise<boolean> {
  const invocationId = messageInvocationID(subscriptionMessageInvocationNamespace, interactiveMessageTool.capabilityId, context, request);
  try {
    const invocation = await client.resolveMcpToolCommand(context.taskId, context.runId, invocationId);
    return invocation.status === "completed";
  } catch {
    return false;
  }
}

function messageProjectionExecutor(
  client: MessageExecutorClient,
  invocationNamespace: string,
  tool: DipoleMcpWriteToolProjection = interactiveMessageTool
): (input: { readonly conversationId: string; readonly content: string }, context: ExecutionContext) => Promise<string> {
  const registry = new CapabilityRegistry();
  registry.register({
    descriptor: {
      id: tool.capabilityId,
      risk: "write",
      requiredPermission: "message.write",
      approvalRequired: true
    },
    inputSchema: messageInputSchema,
    resolveResource: input => ({ resourceType: "conversation", resourceId: input.conversationId, action: "write" }),
    execute: async () => { throw new Error("Agent message writes require an audited Tool Invocation"); }
  });
  const approvals = new McpWriteApprovalGate(
    registry,
    createMcpWriteApprovalConsumePort(client),
    createMcpWriteApprovalGrantResolver(client)
  );
  return (input, context) => new McpMessageWriteProjection(
    approvals,
    new McpToolInvocationRunner(
      { begin: begin => client.begin(begin), finish: finish => client.finishToolInvocation(finish) },
      undefined,
      () => messageInvocationID(invocationNamespace, tool.capabilityId, context, input),
      undefined,
      undefined,
      isUncertainMessageCommandFailure
    ),
    { executeMessageCommand: command => client.executeMessageCommand(command) }
  ).execute(tool, input, context);
}

// Deterministic so a retried reply mints an identical binding: the resolve gate
// looks grants up by (task, capability, scope, argumentsSha256), while the id
// and nonce stay stable across attempts for the same event and reply content.
function subscriptionMessageApproval(
  context: ExecutionContext,
  input: { readonly conversationId: string; readonly content: string; readonly eventId: string; readonly occurredAtUnixMs: number }
): AgentApprovalBinding {
  if (!Number.isSafeInteger(input.occurredAtUnixMs)) throw new Error("Subscription Agent message event time is invalid");
  if (!input.conversationId.trim() || !input.eventId.trim()) throw new Error("Subscription Agent message binding is incomplete");
  const resourceScope = { resourceType: "conversation", resourceId: input.conversationId, actions: ["write"] };
  const argumentsSha256 = messageDigest([canonicalMcpJSON({ conversationId: input.conversationId, content: input.content })]);
  const scopeSha256 = messageDigest(["dipole.agent.scope.v1", resourceScope.resourceType, resourceScope.resourceId, ...resourceScope.actions]);
  const approvalId = `approval:${messageDigest(["dipole.agent.subscription-message.v1", context.taskId, context.runId, input.eventId, argumentsSha256]).slice(0, 48)}`;
  return {
    approvalId,
    capabilityId: "message.system.send",
    resourceScope,
    scopeSha256,
    argumentsSha256,
    nonceSha256: messageDigest(["dipole.agent.subscription-message.nonce.v1", approvalId]),
    expiresAtUnixMs: input.occurredAtUnixMs + 30 * 60 * 1_000
  };
}

function messageDigest(parts: readonly string[]): string {
  return createHash("sha256").update(parts.join("\n"), "utf8").digest("hex");
}

function messageInvocationID(
  invocationNamespace: string,
  capabilityId: string,
  context: ExecutionContext,
  input: { readonly conversationId: string; readonly content: string }
): string {
  const material = [
    invocationNamespace,
    context.taskId,
    context.runId,
    capabilityId,
    input.conversationId,
    input.content
  ].join("\n");
  return `tool:${createHash("sha256").update(material, "utf8").digest("hex").slice(0, 59)}`;
}

function isUncertainMessageCommandFailure(error: unknown): boolean {
  if (typeof error !== "object" || error === null || !("code" in error)) return false;
  const code = (error as { code?: unknown }).code;
  // gRPC DEADLINE_EXCEEDED and UNAVAILABLE can occur after the Core accepted
  // the idempotent command, so a terminal Tool failure would block recovery.
  return code === 4 || code === 14;
}

function directConversationKey(first: string, second: string): string {
  return `direct:${[first.trim(), second.trim()].sort().join(":")}`;
}

function messageActionReference(result: unknown, commandKind: "assistant_reply" | "system_message" | "group_reply"): AgentToolActionReference {
  const reference = z.object({
    resourceType: z.literal("message"),
    resourceId: z.string().trim().min(1).max(64),
    commandKind: z.enum(["assistant_reply", "system_message", "group_reply"]),
    commandId: z.string().trim().min(1).max(128)
  }).strict().parse(result);
  if (reference.commandKind !== commandKind) throw new Error("MCP Message Command kind conflicts with the Tool projection");
  return reference;
}
