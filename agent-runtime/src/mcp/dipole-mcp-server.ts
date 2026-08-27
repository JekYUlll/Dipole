import { McpServer } from "@modelcontextprotocol/server";
import type { z } from "zod";

import type { CapabilityRegistry } from "../capabilities/registry.js";
import { executionContextSchema, type ExecutionContext } from "../runtime/execution-context.js";
import type { McpToolInvocationRunner } from "./mcp-tool-invocation.js";

interface DipoleMcpToolProjectionBase {
  name: string;
  capabilityId: string;
  title: string;
  description: string;
  inputSchema: z.ZodType;
}

export interface DipoleMcpReadToolProjection extends DipoleMcpToolProjectionBase {
  commandKind?: never;
}

export interface DipoleMcpWriteToolProjection extends DipoleMcpToolProjectionBase {
  commandKind: "assistant_reply" | "system_message";
}

export type DipoleMcpToolProjection = DipoleMcpReadToolProjection | DipoleMcpWriteToolProjection;

export interface DipoleMcpWriteExecutor {
  execute(tool: DipoleMcpWriteToolProjection, rawArguments: unknown, context: ExecutionContext): Promise<string>;
}

export function createDipoleMcpServer(input: {
  registry: CapabilityRegistry;
  context: ExecutionContext;
  tools: readonly DipoleMcpToolProjection[];
  runner?: McpToolInvocationRunner;
  writeExecutor?: DipoleMcpWriteExecutor;
}): McpServer {
  const context = executionContextSchema.parse(input.context);
  const descriptors = new Map(input.registry.descriptors().map((descriptor) => [descriptor.id, descriptor]));
  const server = new McpServer({ name: "dipole-agent", version: "0.1.0" });
  const names = new Set<string>();

  for (const tool of input.tools) {
    const name = tool.name.trim();
    const descriptor = descriptors.get(tool.capabilityId);
    if (!/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/.test(name) || names.has(name)) throw new Error(`MCP Tool name ${name} is invalid or duplicated`);
    if (descriptor === undefined) throw new Error(`MCP Tool ${name} must project a registered Capability`);
    const write = descriptor.risk !== "read";
    if (write && (context.mode !== "active" || descriptor.risk !== "write" || descriptor.approvalRequired !== true || tool.commandKind === undefined || input.writeExecutor === undefined)) {
      throw new Error(`MCP Tool ${name} write projection requires an explicit approval-bound executor`);
    }
    if (!write && tool.commandKind !== undefined) throw new Error(`MCP Tool ${name} read projection cannot declare a command kind`);
    names.add(name);
    server.registerTool(name, {
      title: tool.title,
      description: tool.description,
      inputSchema: tool.inputSchema,
      annotations: { readOnlyHint: !write, destructiveHint: false, idempotentHint: true, openWorldHint: false }
    }, async (arguments_) => {
      if (write) {
        const text = await input.writeExecutor!.execute(tool as DipoleMcpWriteToolProjection, arguments_, context);
        return { content: [{ type: "text" as const, text }] };
      }
      const execute = () => input.registry.execute(tool.capabilityId, arguments_, context);
      const text = input.runner === undefined
        ? JSON.stringify(await execute())
        : await input.runner.execute({ name, capabilityId: tool.capabilityId }, arguments_, context, execute);
      if (input.runner === undefined && new TextEncoder().encode(text).length > 64 * 1024) throw new Error(`MCP Tool ${name} result exceeds 64 KiB`);
      return { content: [{ type: "text" as const, text }] };
    });
  }
  return server;
}
