import { createHash } from "node:crypto";
import { z } from "zod";

import type { AgentCapabilityRPCClient, AgentToolActionReference } from "../capabilities/agent-capability-rpc.js";
import { CapabilityRegistry } from "../capabilities/registry.js";
import { canonicalMcpJSON } from "./canonical-json.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
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
    readonly commandKind: "assistant_reply" | "group_reply" | "system_message";
    readonly content: string;
    readonly conversationKey?: string;
    readonly requestId?: string;
    readonly traceId?: string;
  }): Promise<AgentToolActionReference>;
}

const messageInputSchema = z.object({
  conversationId: z.string().trim().min(1).max(256),
  content: z.string().trim().min(1).max(16 * 1024)
}).strict();

type ApprovedMessageTool = Omit<DipoleMcpWriteToolProjection, "commandKind"> & {
  commandKind: "assistant_reply" | "group_reply" | "system_message";
};

export class McpMessageWriteProjection implements DipoleMcpWriteExecutor {
  constructor(
    private readonly approvals: McpWriteApprovalGate,
    private readonly runner: McpToolInvocationRunner,
    private readonly commands: McpMessageCommandPort
  ) {}

  async execute(tool: ApprovedMessageTool, rawArguments: unknown, context: ExecutionContext): Promise<string> {
    const input = messageInputSchema.parse(rawArguments);
    if (tool.commandKind === "group_reply"
      ? !/^group:[^:\s]+$/.test(input.conversationId)
      : input.conversationId !== directConversationKey(context.principalUuid, context.agentUuid)) {
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
          ...(tool.commandKind === "group_reply" ? { conversationKey: input.conversationId } : {}),
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

const approvedGroupMessageTool: ApprovedMessageTool = {
  ...interactiveMessageTool,
  name: "dipole_group_message_send",
  capabilityId: "message.group_reply.send",
  description: "Publish the approved draft to its bound group conversation",
  commandKind: "group_reply"
};

export function createInteractiveMessageExecutor(
  client: Pick<AgentCapabilityRPCClient, "beginMcpToolCommand" | "finishToolInvocation" | "consumeApproval" | "resolveApprovalGrant" | "executeMessageCommand">
): { execute(input: { readonly conversationId: string; readonly content: string }, context: ExecutionContext, approvalId: string): Promise<string> } {
  const registry = new CapabilityRegistry();
  for (const tool of [interactiveMessageTool, approvedGroupMessageTool]) registry.register({
    descriptor: {
      id: tool.capabilityId,
      risk: "write",
      requiredPermission: "message.write",
      approvalRequired: true
    },
    inputSchema: messageInputSchema,
    resolveResource: input => ({ resourceType: "conversation", resourceId: input.conversationId, action: "write" }),
    execute: async () => { throw new Error("Interactive Agent message writes require an audited Tool Invocation"); }
  });
  const grants = createMcpWriteApprovalGrantResolver(client);
  return {
    execute: async (rawInput, context, approvalId) => {
      const input = messageInputSchema.parse(rawInput);
      const tool = input.conversationId.startsWith("group:") ? approvedGroupMessageTool : interactiveMessageTool;
      if (tool.commandKind === "group_reply" ? !/^group:[^:\s]+$/.test(input.conversationId)
        : input.conversationId !== directConversationKey(context.principalUuid, context.agentUuid)) {
        throw new Error("Message destination does not match the task conversation");
      }
      registry.prepare(tool.capabilityId, input, context);
      const invocationId = interactiveMessageInvocationID(context, input);
      const begin = {
        invocationId, taskId: context.taskId, runId: context.runId, toolName: tool.name,
        capabilityId: tool.capabilityId, approvalId,
        argumentsSha256: createHash("sha256").update(canonicalMcpJSON(input)).digest("hex"),
        ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
        ...(context.traceId === undefined ? {} : { traceId: context.traceId })
      };
      let record;
      try {
        // Core restores a consumed approval only for its uniquely bound invocation.
        record = await client.beginMcpToolCommand(begin);
      } catch (error) {
        if (typeof error !== "object" || error === null || !("code" in error) || error.code !== 7) throw error;
        const approvals = new McpWriteApprovalGate(registry, createMcpWriteApprovalConsumePort(client), {
          resolve: async request => {
            const grant = await grants.resolve(request);
            if (grant.approvalId !== approvalId) throw new Error("Approval does not match the workflow binding");
            return grant;
          }
        });
        await approvals.authorize(tool.capabilityId, input, context);
        record = await client.beginMcpToolCommand(begin);
      }
      if (record.status === "failed") throw new Error("Message invocation previously failed");
      const startedAt = performance.now();
      const result = messageActionReference(await client.executeMessageCommand({
        taskId: context.taskId, runId: context.runId, invocationId, commandKind: tool.commandKind,
        content: input.content, ...(tool.commandKind === "group_reply" ? { conversationKey: input.conversationId } : {})
      }), tool.commandKind);
      const canonical = canonicalMcpJSON(result);
      // Leave an uncertain write/audit open. Temporal retries the same command ID.
      if (record.status !== "completed") await client.finishToolInvocation({
        invocationId, taskId: context.taskId, runId: context.runId, status: "completed",
        resultSha256: createHash("sha256").update(canonical).digest("hex"),
        resultBytes: Buffer.byteLength(canonical), latencyMs: Math.max(0, Math.floor(performance.now() - startedAt)), actionReference: result
      });
      return canonical;
    }
  };
}

function interactiveMessageInvocationID(
  context: ExecutionContext,
  input: { readonly conversationId: string; readonly content: string }
): string {
  const material = [
    "dipole.agent.interactive-message-invocation.v1",
    context.taskId,
    context.runId,
    input.conversationId.startsWith("group:") ? approvedGroupMessageTool.capabilityId : interactiveMessageTool.capabilityId,
    input.conversationId,
    input.content
  ].join("\n");
  return `tool:${createHash("sha256").update(material, "utf8").digest("hex").slice(0, 59)}`;
}

function directConversationKey(first: string, second: string): string {
  return `direct:${[first.trim(), second.trim()].sort().join(":")}`;
}

function messageActionReference(result: unknown, commandKind: "assistant_reply" | "group_reply" | "system_message"): AgentToolActionReference {
  const reference = z.object({
    resourceType: z.literal("message"),
    resourceId: z.string().trim().min(1).max(64),
    commandKind: z.enum(["assistant_reply", "group_reply", "system_message"]),
    commandId: z.string().trim().min(1).max(128)
  }).strict().parse(result);
  if (reference.commandKind !== commandKind) throw new Error("MCP Message Command kind conflicts with the Tool projection");
  return reference;
}
