import { createMcpHandler, type AuthInfo, type McpHttpHandler } from "@modelcontextprotocol/server";

import type { CapabilityRegistry } from "../capabilities/registry.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import {
  createDipoleMcpServer,
  type DipoleMcpToolProjection,
  type DipoleMcpWriteExecutor
} from "./dipole-mcp-server.js";
import type { McpToolInvocationRunner } from "./mcp-tool-invocation.js";

export function createDipoleMcpHttpHandler(input: {
  registry: CapabilityRegistry;
  tools: readonly DipoleMcpToolProjection[];
  resolveContext(auth: AuthInfo): ExecutionContext | Promise<ExecutionContext>;
  runner?: McpToolInvocationRunner;
  writeExecutor?: DipoleMcpWriteExecutor;
}): McpHttpHandler {
  return createMcpHandler(async (request) => {
    if (request.authInfo === undefined) throw new Error("authenticated MCP request context is required");
    return createDipoleMcpServer({
      registry: input.registry, tools: input.tools, context: await input.resolveContext(request.authInfo),
      ...(input.runner === undefined ? {} : { runner: input.runner }),
      ...(input.writeExecutor === undefined ? {} : { writeExecutor: input.writeExecutor })
    });
  });
}
