import { describe, expect, it, vi } from "vitest";

import { ConversationSearchCapability } from "./conversation-search.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U1", agentUuid: "UAI", taskId: "T1", runId: "R1", mode: "active",
  permissions: ["conversation.search"], resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["search"] }],
  approvedCapabilities: [], eventId: "E1"
};

describe("ConversationSearchCapability", () => {
  it("uses the trusted execution context and bounded query", async () => {
    const searchConversations = vi.fn().mockResolvedValue({ messages: [] });
    const capability = new ConversationSearchCapability({ searchConversations });

    await expect(capability.execute({ query: "Cassandra", limit: 10 }, context)).resolves.toEqual({ messages: [] });
    expect(searchConversations).toHaveBeenCalledWith(context, "Cassandra", 10);
    expect(capability.resolveResource()).toEqual({ resourceType: "conversation", resourceId: "*", action: "search" });
  });

  it("rejects empty and oversized queries", () => {
    const capability = new ConversationSearchCapability({ searchConversations: vi.fn() });
    expect(() => capability.inputSchema.parse({ query: " " })).toThrow();
    expect(() => capability.inputSchema.parse({ query: "x".repeat(513) })).toThrow();
  });
});
