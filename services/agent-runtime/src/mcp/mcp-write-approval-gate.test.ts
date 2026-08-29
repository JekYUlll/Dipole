import { createHash } from "node:crypto";

import { describe, expect, it, vi } from "vitest";
import { z } from "zod";

import { CapabilityRegistry } from "../capabilities/registry.js";
import { executionContextSchema, type ExecutionContext } from "../runtime/execution-context.js";
import { createMcpWriteApprovalGrantResolver, McpWriteApprovalGate } from "./mcp-write-approval-gate.js";

describe("MCP write Approval gate", () => {
  it("consumes an exact durable binding before one write execution", async () => {
    const execute = vi.fn(async () => ({ messageId: "M1" }));
    const registry = writeRegistry(execute);
    const consume = vi.fn(async () => undefined);
    const resolve = vi.fn(async () => grant({ content: "notice", conversationId: "group:G1" }));
    const gate = new McpWriteApprovalGate(registry, { consume }, { resolve });

    await expect(gate.execute("message.system.send", {
      conversationId: "group:G1", content: "notice"
    }, context())).resolves.toEqual({ messageId: "M1" });

    expect(consume).toHaveBeenCalledOnce();
    expect(consume).toHaveBeenCalledWith({
      taskId: "TASK-1", runId: "RUN-1", approvalId: "APR-1", capabilityId: "message.system.send",
      scopeSha256: scopeHash(), argumentsSha256: argumentHash({ content: "notice", conversationId: "group:G1" }),
      nonceSha256: sha256("host-owned-once-nonce"), requestId: "REQ-1", traceId: "TRACE-1"
    });
    expect(execute).toHaveBeenCalledOnce();
    expect(scopeHash()).toBe("dc6c631b043f8f54bec8a9c5376b06a859cb967923b3efe59b1d57f216a00ab0");
  });

  it("fails before consumption or execution for shadow, read and binding drift", async () => {
    const execute = vi.fn(async () => ({ ok: true }));
    const registry = writeRegistry(execute);
    registry.register({
      descriptor: { id: "conversation.list", risk: "read", requiredPermission: "message.write" },
      inputSchema: z.object({}).strict(), resolveResource: () => ({ resourceType: "conversation", resourceId: "group:G1", action: "write" }), execute
    });
    const consume = vi.fn(async () => undefined);
    const resolve = vi.fn(async () => grant({ content: "different", conversationId: "group:G1" }));
    const gate = new McpWriteApprovalGate(registry, { consume }, { resolve });

    await expect(gate.execute("message.system.send", {
      conversationId: "group:G1", content: "notice"
    }, { ...context(), mode: "shadow" })).rejects.toThrow(/active/i);
    await expect(gate.execute("conversation.list", {}, context())).rejects.toThrow(/write/i);
    await expect(gate.execute("message.system.send", {
      conversationId: "group:G1", content: "notice"
    }, context())).rejects.toThrow(/binding/i);
    expect(consume).not.toHaveBeenCalled();
    expect(execute).not.toHaveBeenCalled();
  });

  it("does not execute when atomic consumption rejects a replay", async () => {
    const execute = vi.fn(async () => ({ ok: true }));
    const consume = vi.fn(async () => { throw new Error("approval denied"); });
    const gate = new McpWriteApprovalGate(writeRegistry(execute), { consume }, {
      resolve: async () => grant({ content: "notice", conversationId: "group:G1" })
    });

    await expect(gate.execute("message.system.send", {
      conversationId: "group:G1", content: "notice"
    }, context())).rejects.toThrow(/unavailable/i);
    expect(execute).not.toHaveBeenCalled();
  });

  it("keeps the Approval consumed when the write operation fails", async () => {
    const execute = vi.fn(async () => { throw new Error("command unavailable"); });
    const consume = vi.fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("already consumed"));
    const gate = new McpWriteApprovalGate(writeRegistry(execute), { consume }, {
      resolve: async () => grant({ content: "notice", conversationId: "group:G1" })
    });
    const invoke = () => gate.execute("message.system.send", {
      conversationId: "group:G1", content: "notice"
    }, context());

    await expect(invoke()).rejects.toThrow(/command unavailable/i);
    await expect(invoke()).rejects.toThrow(/already consumed/i);
    expect(execute).toHaveBeenCalledOnce();
  });

  it("resolves the persisted exact grant through the authenticated RPC port", async () => {
    const resolveApprovalGrant = vi.fn(async () => grant({ content: "notice", conversationId: "group:G1" }));
    const resolver = createMcpWriteApprovalGrantResolver({ resolveApprovalGrant });

    await expect(resolver.resolve({
      capabilityId: "message.system.send",
      resource: { resourceType: "conversation", resourceId: "group:G1", action: "write" },
      arguments: { conversationId: "group:G1", content: "notice" },
      context: context()
    })).resolves.toMatchObject({ approvalId: "APR-1", nonceSha256: expect.stringMatching(/^[a-f0-9]{64}$/) });
    expect(resolveApprovalGrant).toHaveBeenCalledWith(
      "TASK-1", "RUN-1", "message.system.send",
      { resourceType: "conversation", resourceId: "group:G1", actions: ["write"] },
      argumentHash({ content: "notice", conversationId: "group:G1" }),
      { requestId: "REQ-1", traceId: "TRACE-1" }
    );
  });
});

function writeRegistry(execute: (input: { conversationId: string; content: string }, context: ExecutionContext) => Promise<unknown>): CapabilityRegistry {
  const registry = new CapabilityRegistry();
  registry.register({
    descriptor: { id: "message.system.send", risk: "write", requiredPermission: "message.write", approvalRequired: true },
    inputSchema: z.object({ conversationId: z.string(), content: z.string() }).strict(),
    resolveResource: input => ({ resourceType: "conversation", resourceId: input.conversationId, action: "write" }),
    execute
  });
  return registry;
}

function context() {
  return executionContextSchema.parse({
    tenantId: "dipole", principalUuid: "U100", agentUuid: "AI1", taskId: "TASK-1", runId: "RUN-1", mode: "active",
    permissions: ["message.write"], resourceScopes: [{ resourceType: "conversation", resourceId: "group:G1", actions: ["write"] }],
    approvedCapabilities: ["message.system.send"], requestId: "REQ-1", traceId: "TRACE-1"
  });
}

function grant(arguments_: Record<string, unknown>) {
  return {
    approvalId: "APR-1", capabilityId: "message.system.send",
    resourceScope: { resourceType: "conversation", resourceId: "group:G1", actions: ["write"] },
    scopeSha256: scopeHash(), argumentsSha256: argumentHash(arguments_), nonceSha256: sha256("host-owned-once-nonce"),
    expiresAtUnixMs: Date.now() + 60_000
  };
}

function scopeHash(): string {
  return sha256("dipole.agent.scope.v1\nconversation\ngroup:G1\nwrite");
}

function argumentHash(value: unknown): string {
  return sha256(JSON.stringify(value, Object.keys(value as object).sort()));
}

function sha256(value: string): string {
  return createHash("sha256").update(value).digest("hex");
}
