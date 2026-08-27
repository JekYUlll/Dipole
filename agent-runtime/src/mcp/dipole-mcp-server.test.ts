import { InMemoryTransport } from "@modelcontextprotocol/server";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/client";
import { describe, expect, it, vi } from "vitest";
import { z } from "zod";

import { CapabilityRegistry } from "../capabilities/registry.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import { createDipoleMcpServer } from "./dipole-mcp-server.js";
import { createDipoleMcpHttpHandler } from "./dipole-mcp-http.js";
import { AllowlistedMcpToolClient } from "./mcp-tool-client.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "AI1", taskId: "TASK-1", runId: "RUN-1", mode: "shadow",
  permissions: ["conversation.list"], resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["list"] }], approvedCapabilities: []
};

describe("Dipole MCP read-only projection", () => {
  it("injects trusted context and exposes only explicitly projected read Capabilities", async () => {
    const execute = vi.fn(async (_input: { limit: number }, ctx: ExecutionContext) => ({ owner: ctx.principalUuid, count: 1 }));
    const registry = new CapabilityRegistry();
    registry.register({
      descriptor: { id: "conversation.list", risk: "read", requiredPermission: "conversation.list" },
      inputSchema: z.object({ limit: z.number().int().min(1).max(100) }).strict(),
      resolveResource: () => ({ resourceType: "conversation", resourceId: "*", action: "list" }), execute
    });
    const server = createDipoleMcpServer({ registry, context, tools: [{
      name: "dipole_conversation_list", capabilityId: "conversation.list", title: "List conversations",
      description: "List conversations visible to the authenticated Task principal", inputSchema: z.object({ limit: z.number().int().min(1).max(100) }).strict()
    }] });
    const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
    await server.connect(serverTransport);
    const client = new AllowlistedMcpToolClient("dipole-agent", ["dipole-agent"], ["dipole_conversation_list"]);
    await expect(client.connect(clientTransport)).resolves.toHaveLength(1);
    const result = await client.callTool("dipole_conversation_list", { limit: 10 });
    expect(result.content).toEqual([{ type: "text", text: JSON.stringify({ owner: "U100", count: 1 }) }]);
    expect(execute).toHaveBeenCalledWith({ limit: 10 }, expect.objectContaining({ principalUuid: "U100" }));
    await expect(client.callTool("dipole_conversation_list", { limit: 10, principalUuid: "U999" })).rejects.toThrow();
    await expect(client.callTool("unlisted", {})).rejects.toThrow(/allowlisted/);
    await client.close();
    await server.close();
  });

  it("rejects write projections and non-allowlisted servers before connection", () => {
    const registry = new CapabilityRegistry();
    registry.register({
      descriptor: { id: "message.send", risk: "write", requiredPermission: "message.send" }, inputSchema: z.object({}),
      resolveResource: () => ({ resourceType: "conversation", resourceId: "G1", action: "write" }), execute: async () => ({})
    });
    expect(() => createDipoleMcpServer({ registry, context, tools: [{
      name: "send", capabilityId: "message.send", title: "Send", description: "Send", inputSchema: z.object({})
    }] })).toThrow(/read-only/);
    expect(() => new AllowlistedMcpToolClient("unknown", ["dipole-agent"], ["read"])).toThrow(/Server/);
  });

  it("rejects a configured identity that differs from the MCP handshake", async () => {
    const server = createDipoleMcpServer({ registry: new CapabilityRegistry(), context, tools: [] });
    const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
    await server.connect(serverTransport);
    const client = new AllowlistedMcpToolClient("expected-server", ["expected-server"], ["read"]);
    await expect(client.connect(clientTransport)).rejects.toThrow(/identity mismatch/);
    await server.close();
  });

  it("serves Streamable HTTP only with host-validated authentication context", async () => {
    const registry = new CapabilityRegistry();
    registry.register({
      descriptor: { id: "conversation.list", risk: "read", requiredPermission: "conversation.list" },
      inputSchema: z.object({ limit: z.number().int().min(1).max(100) }).strict(),
      resolveResource: () => ({ resourceType: "conversation", resourceId: "*", action: "list" }),
      execute: async (_input, execution) => ({ principal: execution.principalUuid })
    });
    const handler = createDipoleMcpHttpHandler({ registry, resolveContext: (auth) => ({ ...context, principalUuid: auth.clientId }), tools: [{
      name: "dipole_conversation_list", capabilityId: "conversation.list", title: "List conversations",
      description: "List conversations", inputSchema: z.object({ limit: z.number().int().min(1).max(100) }).strict()
    }] });
    const transport = new StreamableHTTPClientTransport(new URL("http://dipole.test/mcp"), {
      fetch: (url, init) => handler.fetch(new Request(url, init), {
        authInfo: { token: "test-token", clientId: "U200", scopes: ["conversation.list"] }
      })
    });
    const client = new AllowlistedMcpToolClient("dipole-agent", ["dipole-agent"], ["dipole_conversation_list"]);
    await client.connect(transport);
    await expect(client.callTool("dipole_conversation_list", { limit: 1 })).resolves.toMatchObject({
      content: [{ type: "text", text: JSON.stringify({ principal: "U200" }) }]
    });
    await client.close();
    await handler.close();
  });
});
