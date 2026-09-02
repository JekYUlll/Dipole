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
    readonly commandKind: "assistant_reply" | "system_message";
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
    if (input.conversationId !== directConversationKey(context.principalUuid, context.agentUuid)) {
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
type SubscriptionMessageExecutorClient = MessageExecutorClient & Pick<AgentCapabilityRPCClient, "authorizeSubscriptionMessage">;

const interactiveMessageInvocationNamespace = "dipole.agent.interactive-message-invocation.v1";
const subscriptionMessageInvocationNamespace = "dipole.agent.subscription-message-invocation.v1";

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
      await client.authorizeSubscriptionMessage(context.taskId, context.runId, binding, context);
      return projection(request, context);
    }
  };
}

function messageProjectionExecutor(
  client: MessageExecutorClient,
  invocationNamespace: string
): (input: { readonly conversationId: string; readonly content: string }, context: ExecutionContext) => Promise<string> {
  const registry = new CapabilityRegistry();
  registry.register({
    descriptor: {
      id: "message.system.send",
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
      () => messageInvocationID(invocationNamespace, context, input),
      undefined,
      undefined,
      isUncertainMessageCommandFailure
    ),
    { executeMessageCommand: command => client.executeMessageCommand(command) }
  ).execute(interactiveMessageTool, input, context);
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
  context: ExecutionContext,
  input: { readonly conversationId: string; readonly content: string }
): string {
  const material = [
    invocationNamespace,
    context.taskId,
    context.runId,
    interactiveMessageTool.capabilityId,
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

function messageActionReference(result: unknown, commandKind: "assistant_reply" | "system_message"): AgentToolActionReference {
  const reference = z.object({
    resourceType: z.literal("message"),
    resourceId: z.string().trim().min(1).max(64),
    commandKind: z.enum(["assistant_reply", "system_message"]),
    commandId: z.string().trim().min(1).max(128)
  }).strict().parse(result);
  if (reference.commandKind !== commandKind) throw new Error("MCP Message Command kind conflicts with the Tool projection");
  return reference;
}
