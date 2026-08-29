import { describe, expect, it, vi } from "vitest";

import type { ConversationReadResult } from "./agent-capability-rpc.js";
import { ConversationReadCapability } from "./conversation-read.js";
import { CapabilityRegistry } from "./registry.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1", mode: "shadow",
  permissions: ["conversation.read"],
  resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read"] }],
  approvedCapabilities: []
};

describe("ConversationReadCapability", () => {
  it("validates input and forwards the trusted execution context", async () => {
    const result: ConversationReadResult = { found: true, reason: "", targetId: "G1", targetType: 2, messages: [] };
    const readConversation = vi.fn(async (receivedContext: ExecutionContext, conversationId: string, limit: number) => {
      expect(receivedContext).toBe(context);
      expect(conversationId).toBe("group:G1");
      expect(limit).toBe(20);
      return result;
    });
    const capability = new ConversationReadCapability({ readConversation });

    await expect(capability.execute(capability.inputSchema.parse({ conversationId: " group:G1 " }), context)).resolves.toBe(result);
    expect(readConversation).toHaveBeenCalledOnce();
  });

  it("requires the conversation.read permission before remote execution", async () => {
    const readConversation = vi.fn();
    const capability = new ConversationReadCapability({ readConversation });
    const denied = { ...context, permissions: ["conversation.list"] } as ExecutionContext;
    const registry = new CapabilityRegistry();
    registry.register(capability);

    await expect(registry.execute("conversation.read", { conversationId: "group:G1" }, denied)).rejects.toThrow(/missing permission/);
    expect(readConversation).not.toHaveBeenCalled();
  });
});
