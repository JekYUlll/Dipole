import { McpServer } from "@modelcontextprotocol/server";
import type { z } from "zod";

import type { CapabilityRegistry } from "../capabilities/registry.js";
import { executionContextSchema, type ExecutionContext } from "../runtime/execution-context.js";

export interface DipoleMcpToolProjection {
  name: string;
  capabilityId: string;
  title: string;
  description: string;
  inputSchema: z.ZodType;
}

export function createDipoleMcpServer(input: {
  registry: CapabilityRegistry;
  context: ExecutionContext;
  tools: readonly DipoleMcpToolProjection[];
}): McpServer {
  const context = executionContextSchema.parse(input.context);
  const descriptors = new Map(input.registry.descriptors().map((descriptor) => [descriptor.id, descriptor]));
  const server = new McpServer({ name: "dipole-agent", version: "0.1.0" });
  const names = new Set<string>();

  for (const tool of input.tools) {
    const name = tool.name.trim();
    const descriptor = descriptors.get(tool.capabilityId);
    if (!/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/.test(name) || names.has(name)) throw new Error(`MCP Tool name ${name} is invalid or duplicated`);
    if (descriptor === undefined || descriptor.risk !== "read") throw new Error(`MCP Tool ${name} must project a registered read-only Capability`);
    names.add(name);
    server.registerTool(name, {
      title: tool.title,
      description: tool.description,
      inputSchema: tool.inputSchema,
      annotations: { readOnlyHint: true, destructiveHint: false, idempotentHint: true, openWorldHint: false }
    }, async (arguments_) => {
      const result = await input.registry.execute(tool.capabilityId, arguments_, context);
      const text = JSON.stringify(result);
      if (new TextEncoder().encode(text).length > 64 * 1024) throw new Error(`MCP Tool ${name} result exceeds 64 KiB`);
      return { content: [{ type: "text" as const, text }] };
    });
  }
  return server;
}
