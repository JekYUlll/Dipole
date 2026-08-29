import { z } from "zod";

import type { AgentToolActionReference } from "../capabilities/agent-capability-rpc.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import type { DipoleMcpWriteExecutor, DipoleMcpWriteToolProjection } from "./dipole-mcp-server.js";
import type { McpToolInvocationRunner } from "./mcp-tool-invocation.js";
import type { McpWriteApprovalGate } from "./mcp-write-approval-gate.js";

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
