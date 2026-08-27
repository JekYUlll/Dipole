import { createMcpHandler, type AuthInfo, type McpHttpHandler } from "@modelcontextprotocol/server";

import type { CapabilityRegistry } from "../capabilities/registry.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import { createDipoleMcpServer, type DipoleMcpToolProjection } from "./dipole-mcp-server.js";

export function createDipoleMcpHttpHandler(input: {
  registry: CapabilityRegistry;
  tools: readonly DipoleMcpToolProjection[];
  resolveContext(auth: AuthInfo): ExecutionContext;
}): McpHttpHandler {
  return createMcpHandler((request) => {
    if (request.authInfo === undefined) throw new Error("authenticated MCP request context is required");
    return createDipoleMcpServer({ registry: input.registry, tools: input.tools, context: input.resolveContext(request.authInfo) });
  });
}
