import { describe, expect, it, vi } from "vitest";

import { ConversationSearchCapability } from "./conversation-search.js";

const context = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1", mode: "shadow" as const,
  permissions: ["conversation.search"], resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read"] }], approvedCapabilities: []
};

describe("ConversationSearchCapability", () => {
  it("uses only the wildcard read resource and forwards bounded input", async () => {
    const searchConversations = vi.fn(async () => []);
    const capability = new ConversationSearchCapability({ searchConversations });
    expect(capability.resolveResource({ query: "migration", limit: 10 }, context)).toEqual({ resourceType: "conversation", resourceId: "*", action: "read" });
    await expect(capability.execute({ query: "migration", limit: 10 }, context)).resolves.toEqual([]);
    expect(searchConversations).toHaveBeenCalledWith(context, "migration", 10);
  });

  it("rejects empty and oversized query input before RPC", () => {
    const capability = new ConversationSearchCapability({ searchConversations: vi.fn() });
    expect(() => capability.inputSchema.parse({ query: "" })).toThrow();
    expect(() => capability.inputSchema.parse({ query: "x".repeat(257) })).toThrow();
    expect(() => capability.inputSchema.parse({ query: "migration", limit: 21 })).toThrow();
  });
});
