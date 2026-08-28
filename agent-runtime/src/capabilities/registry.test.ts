import { describe, expect, it, vi } from "vitest";

import { CapabilityRegistry } from "./registry.js";
import { executionContextSchema } from "../runtime/execution-context.js";

describe("CapabilityRegistry", () => {
  it("rejects duplicate IDs and authorizes before executing", async () => {
    const registry = new CapabilityRegistry();
    const execute = vi.fn(async (input: { conversationId: string }) => ({ found: input.conversationId === "group:G1" }));
    const capability = {
      descriptor: { id: "conversation.read", risk: "read" as const, requiredPermission: "conversation.read" },
      inputSchema: { parse: (input: unknown) => input as { conversationId: string } },
      resolveResource: (input: { conversationId: string }) => ({ resourceType: "conversation", resourceId: input.conversationId, action: "read" }),
      execute
    };
    registry.register(capability);
    expect(() => registry.register(capability)).toThrow(/already registered/);

    const context = executionContextSchema.parse({
      tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1", mode: "active",
      permissions: ["conversation.read"],
      resourceScopes: [{ resourceType: "conversation", resourceId: "group:G1", actions: ["read"] }],
      approvedCapabilities: []
    });
    await expect(registry.execute("conversation.read", { conversationId: "group:G1" }, context)).resolves.toEqual({ found: true });
    await expect(registry.execute("conversation.read", { conversationId: "group:G2" }, context)).rejects.toThrow(/scope/);
    expect(execute).toHaveBeenCalledTimes(1);
  });

  it("rejects unbounded or non-schema fields before exposing descriptors", () => {
    const capability = (inputSchema: Record<string, unknown>) => ({
      descriptor: { id: "conversation.read", risk: "read" as const, requiredPermission: "conversation.read", inputSchema },
      inputSchema: { parse: (input: unknown) => input },
      resolveResource: () => ({ resourceType: "conversation", resourceId: "*", action: "read" }),
      execute: async () => undefined
    });

    expect(() => new CapabilityRegistry().register(capability({ type: "object", description: "leak" }))).toThrow(/key description/);
    expect(() => new CapabilityRegistry().register(capability({ type: "object", properties: { value: { type: "string" } }, x: true }))).toThrow(/key x/);
    expect(() => new CapabilityRegistry().register(capability({ type: "object", default: "x".repeat(5 * 1024) }))).toThrow(/too large/);
  });
});
