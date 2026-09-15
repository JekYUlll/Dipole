import { createHash } from "node:crypto";
import { z } from "zod";

import type { AgentCapabilityRPCClient, AgentContextMemory } from "../capabilities/agent-capability-rpc.js";
import { CapabilityRegistry } from "../capabilities/registry.js";
import { canonicalMcpJSON } from "./canonical-json.js";
import { McpToolInvocationRunner } from "./mcp-tool-invocation.js";
import { createMcpWriteApprovalConsumePort, createMcpWriteApprovalGrantResolver, McpWriteApprovalGate } from "./mcp-write-approval-gate.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const memoryInputSchema = z.object({
  content: z.string().trim().min(1).max(1000),
  conversationId: z.string().trim().min(1).max(256),
  memoryType: z.enum(["semantic", "episodic"])
}).strict();

const memoryTool = {
  name: "dipole_memory_save",
  capabilityId: "memory.save",
  requiredPermission: "memory.write",
  inputSchema: memoryInputSchema
} as const;

export function createInteractiveMemoryExecutor(
  client: Pick<AgentCapabilityRPCClient, "beginMcpToolCommand" | "finishToolInvocation" | "consumeApproval" | "resolveApprovalGrant" | "executeMemoryCommand">
): { execute(input: z.infer<typeof memoryInputSchema>, context: ExecutionContext, approvalId: string): Promise<AgentContextMemory> } {
  const registry = new CapabilityRegistry();
  registry.register({
    descriptor: { id: memoryTool.capabilityId, risk: "write", requiredPermission: memoryTool.requiredPermission, approvalRequired: true },
    inputSchema: memoryInputSchema,
    resolveResource: input => ({ resourceType: "conversation", resourceId: input.conversationId, action: "write" }),
    execute: async () => { throw new Error("Interactive Memory writes require an audited Tool Invocation"); }
  });
  const grants = createMcpWriteApprovalGrantResolver(client);
  return {
    execute: async (rawInput, context, approvalId) => {
      const input = memoryInputSchema.parse(rawInput);
      registry.prepare(memoryTool.capabilityId, input, context);
      const invocationId = memoryInvocationID(context, input);
      const begin = {
        invocationId, taskId: context.taskId, runId: context.runId, toolName: memoryTool.name,
        capabilityId: memoryTool.capabilityId, approvalId,
        argumentsSha256: createHash("sha256").update(canonicalMcpJSON(input)).digest("hex"),
        ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
        ...(context.traceId === undefined ? {} : { traceId: context.traceId })
      };
      let record;
      try {
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
        await approvals.authorize(memoryTool.capabilityId, input, context);
        record = await client.beginMcpToolCommand(begin);
      }
      if (record.status === "failed") throw new Error("Memory invocation previously failed");
      const startedAt = performance.now();
      const memory = await client.executeMemoryCommand({
        taskId: context.taskId, runId: context.runId, invocationId, memoryType: input.memoryType,
        content: input.content, conversationKey: input.conversationId,
        ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
        ...(context.traceId === undefined ? {} : { traceId: context.traceId })
      });
      const result = canonicalMcpJSON({ memoryId: memory.memoryId });
      if (record.status !== "completed") await client.finishToolInvocation({
        invocationId, taskId: context.taskId, runId: context.runId, status: "completed",
        resultSha256: createHash("sha256").update(result).digest("hex"), resultBytes: Buffer.byteLength(result),
        latencyMs: Math.max(0, Math.floor(performance.now() - startedAt))
      });
      return memory;
    }
  };
}

function memoryInvocationID(context: ExecutionContext, input: z.infer<typeof memoryInputSchema>): string {
  const material = ["dipole.agent.memory-invocation.v1", context.taskId, context.runId, input.conversationId, input.memoryType, input.content].join("\n");
  return `tool:${createHash("sha256").update(material, "utf8").digest("hex").slice(0, 59)}`;
}
