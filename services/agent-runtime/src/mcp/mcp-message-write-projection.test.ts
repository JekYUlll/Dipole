import { InMemoryTransport } from "@modelcontextprotocol/server";
import { describe, expect, it, vi } from "vitest";
import { z } from "zod";
import * as grpc from "@grpc/grpc-js";

import { CapabilityRegistry } from "../capabilities/registry.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import { createDipoleMcpServer } from "./dipole-mcp-server.js";
import { createInteractiveMessageExecutor, createInteractiveReplyExecutor, createSubscriptionMessageExecutor, McpMessageWriteProjection, subscriptionReplyReplayMarker } from "./mcp-message-write-projection.js";
import { AllowlistedMcpToolClient } from "./mcp-tool-client.js";
import { McpToolInvocationRunner } from "./mcp-tool-invocation.js";
import { McpWriteApprovalGate } from "./mcp-write-approval-gate.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1", mode: "active",
  permissions: ["message.write"], resourceScopes: [{ resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] }],
  approvedCapabilities: ["message.system.send"], requestId: "REQ-1", traceId: "TRACE-1"
};

describe("MCP Message write projection", () => {
  it("binds one consumed Approval, Tool Invocation and Message Command receipt", async () => {
    const order: string[] = [];
    const registry = writeRegistry();
    const consume = vi.fn(async () => { order.push("consume"); });
    const gate = new McpWriteApprovalGate(registry, { consume }, {
      resolve: async () => ({
        approvalId: "APR-1", capabilityId: "message.system.send",
        resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] },
        scopeSha256: "fda03fe9202766c3e59c11b4b069749a400e50041a44d1d200ff41c64aefa8a5",
        argumentsSha256: "5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9",
        nonceSha256: "d".repeat(64), expiresAtUnixMs: Date.now() + 60_000
      })
    });
    const begin = vi.fn(async () => { order.push("begin"); });
    const finish = vi.fn(async () => { order.push("finish"); });
    const executeMessageCommand = vi.fn(async (input: { invocationId: string }) => {
      order.push("command");
      expect(input.invocationId).toBe("INV-WRITE-1");
      return { resourceType: "message" as const, resourceId: "MSG-1", commandKind: "system_message" as const, commandId: "tool:command-1" };
    });
    const projection = new McpMessageWriteProjection(
      gate,
      new McpToolInvocationRunner({ begin, finish }, undefined, () => "INV-WRITE-1"),
      { executeMessageCommand }
    );
    const server = createDipoleMcpServer({ registry, context, writeExecutor: projection, tools: [{
      name: "dipole_message_send", capabilityId: "message.system.send", title: "Send message",
      description: "Send one approved system message", inputSchema: messageInputSchema, commandKind: "system_message"
    }] });
    const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
    await server.connect(serverTransport);
    const client = new AllowlistedMcpToolClient("dipole-agent", ["dipole-agent"], ["dipole_message_send"], {
      dipole_message_send: { allowedArgumentNames: ["conversationId", "content"], maximumBytes: 1024 }
    });
    await client.connect(clientTransport);

    const result = await client.callTool("dipole_message_send", { conversationId: "direct:U100:UAI", content: "notice" });

    expect(order).toEqual(["consume", "begin", "command", "finish"]);
    expect(executeMessageCommand).toHaveBeenCalledWith({
      taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-WRITE-1", commandKind: "system_message",
      content: "notice", requestId: "REQ-1", traceId: "TRACE-1"
    });
    expect(begin).toHaveBeenCalledWith(expect.objectContaining({ approvalId: "APR-1", invocationId: "INV-WRITE-1" }));
    expect(finish).toHaveBeenCalledWith(expect.objectContaining({
      status: "completed",
      actionReference: { resourceType: "message", resourceId: "MSG-1", commandKind: "system_message", commandId: "tool:command-1" }
    }));
    expect(result.content).toEqual([{ type: "text", text: JSON.stringify({
      commandId: "tool:command-1", commandKind: "system_message", resourceId: "MSG-1", resourceType: "message"
    }) }]);
    await client.close();
    await server.close();
  });

  it("rejects a conversation drift before consuming Approval or beginning audit", async () => {
    const consume = vi.fn(async () => undefined);
    const begin = vi.fn(async () => undefined);
    const gate = new McpWriteApprovalGate(writeRegistry(), { consume }, {
      resolve: vi.fn(async () => { throw new Error("must not resolve"); })
    });
    const projection = new McpMessageWriteProjection(
      gate,
      new McpToolInvocationRunner({ begin, finish: vi.fn(async () => undefined) }),
      { executeMessageCommand: vi.fn() }
    );

    await expect(projection.execute({
      name: "dipole_message_send", capabilityId: "message.system.send", title: "Send message",
      description: "Send one approved system message", inputSchema: messageInputSchema, commandKind: "system_message"
    }, { conversationId: "group:G1", content: "notice" }, context)).rejects.toThrow(/direct conversation/);
    expect(consume).not.toHaveBeenCalled();
    expect(begin).not.toHaveBeenCalled();
  });

  it("rejects a conflicting Message action reference before completing audit", async () => {
    const finish = vi.fn(async () => undefined);
    const gate = new McpWriteApprovalGate(writeRegistry(), { consume: vi.fn(async () => undefined) }, {
      resolve: async () => ({
        approvalId: "APR-1", capabilityId: "message.system.send",
        resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] },
        scopeSha256: "fda03fe9202766c3e59c11b4b069749a400e50041a44d1d200ff41c64aefa8a5",
        argumentsSha256: "5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9",
        nonceSha256: "d".repeat(64), expiresAtUnixMs: Date.now() + 60_000
      })
    });
    const projection = new McpMessageWriteProjection(
      gate,
      new McpToolInvocationRunner({ begin: vi.fn(async () => undefined), finish }, undefined, () => "INV-DRIFT"),
      { executeMessageCommand: async () => ({
        resourceType: "message", resourceId: "MSG-1", commandKind: "assistant_reply", commandId: "tool:drift"
      }) }
    );

    await expect(projection.execute({
      name: "dipole_message_send", capabilityId: "message.system.send", title: "Send message",
      description: "Send one approved system message", inputSchema: messageInputSchema, commandKind: "system_message"
    }, { conversationId: "direct:U100:UAI", content: "notice" }, context)).rejects.toThrow(/Tool invocation failed/);
    expect(finish).toHaveBeenCalledWith(expect.objectContaining({ status: "failed", errorCode: "tool_execution_failed" }));
    expect(finish).not.toHaveBeenCalledWith(expect.objectContaining({ status: "completed" }));
  });

  it("composes the interactive executor from the approval, audit, and Core command ports", async () => {
    const order: string[] = [];
    const client = {
      consumeApproval: vi.fn(async () => { order.push("consume"); }),
      resolveApprovalGrant: vi.fn(async () => ({
        approvalId: "APR-1", capabilityId: "message.system.send",
        resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] },
        scopeSha256: "fda03fe9202766c3e59c11b4b069749a400e50041a44d1d200ff41c64aefa8a5",
        argumentsSha256: "5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9",
        nonceSha256: "d".repeat(64), expiresAtUnixMs: Date.now() + 60_000
      })),
      begin: vi.fn(async () => { order.push("begin"); }),
      finishToolInvocation: vi.fn(async () => { order.push("finish"); }),
      executeMessageCommand: vi.fn(async () => {
        order.push("command");
        return { resourceType: "message" as const, resourceId: "MSG-INTERACTIVE-1", commandKind: "system_message" as const, commandId: "tool:interactive-1" };
      })
    };
    const result = await createInteractiveMessageExecutor(client).execute({
      conversationId: "direct:U100:UAI", content: "notice"
    }, context);

    expect(order).toEqual(["consume", "begin", "command", "finish"]);
    expect(result).toBe(JSON.stringify({
      commandId: "tool:interactive-1", commandKind: "system_message", resourceId: "MSG-INTERACTIVE-1", resourceType: "message"
    }));
  });

  it("mints a subscription-scoped grant and uses a distinct Tool Invocation namespace", async () => {
    const invocationIds: string[] = [];
    const makeClient = () => ({
      authorizeSubscriptionMessage: vi.fn(async (_taskId: string, _runId: string, _binding: {
        approvalId: string; capabilityId: string;
        resourceScope: { resourceType: string; resourceId: string; actions: string[] };
        scopeSha256: string; argumentsSha256: string; nonceSha256: string; expiresAtUnixMs: number;
      }) => undefined),
      consumeApproval: vi.fn(async () => undefined),
      resolveApprovalGrant: vi.fn(async () => ({
        approvalId: "APR-1", capabilityId: "message.system.send",
        resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] },
        scopeSha256: "fda03fe9202766c3e59c11b4b069749a400e50041a44d1d200ff41c64aefa8a5",
        argumentsSha256: "5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9",
        nonceSha256: "d".repeat(64), expiresAtUnixMs: Date.now() + 60_000
      })),
      begin: vi.fn(async () => undefined),
      finishToolInvocation: vi.fn(async () => undefined),
      resolveMcpToolCommand: vi.fn(async () => { throw new Error("no completed invocation"); }),
      executeMessageCommand: vi.fn(async (input: { invocationId: string }) => {
        invocationIds.push(input.invocationId);
        return { resourceType: "message" as const, resourceId: "MSG-1", commandKind: "system_message" as const, commandId: `tool:${input.invocationId}` };
      })
    });

    const interactiveClient = makeClient();
    await createInteractiveMessageExecutor(interactiveClient).execute({ conversationId: "direct:U100:UAI", content: "notice" }, context);

    const subscriptionClient = makeClient();
    await createSubscriptionMessageExecutor(subscriptionClient).execute(
      { conversationId: "direct:U100:UAI", content: "notice", eventId: "E-SUB-1", occurredAtUnixMs: 1_700_000_000_000 }, context
    );

    expect(subscriptionClient.authorizeSubscriptionMessage).toHaveBeenCalledOnce();
    const mintArgs = subscriptionClient.authorizeSubscriptionMessage.mock.calls[0]!;
    expect(mintArgs[0]).toBe("TASK-1");
    expect(mintArgs[1]).toBe("RUN-1");
    expect(mintArgs[2]).toMatchObject({
      capabilityId: "message.system.send",
      resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] },
      argumentsSha256: "5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9",
      expiresAtUnixMs: 1_700_000_000_000 + 30 * 60 * 1_000
    });
    expect(mintArgs[2].approvalId).toMatch(/^approval:[0-9a-f]{48}$/);
    expect(interactiveClient.authorizeSubscriptionMessage).not.toHaveBeenCalled();

    expect(invocationIds).toHaveLength(2);
    expect(invocationIds[0]).not.toBe(invocationIds[1]);
  });

  it("mints an owner-scoped assistant_reply grant via AuthorizeInteractiveReply", async () => {
    const invocationIds: string[] = [];
    const client = {
      authorizeInteractiveReply: vi.fn(async () => undefined),
      consumeApproval: vi.fn(async () => undefined),
      resolveApprovalGrant: vi.fn(async () => ({
        approvalId: "APR-1", capabilityId: "message.assistant_reply.send",
        resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] },
        scopeSha256: "fda03fe9202766c3e59c11b4b069749a400e50041a44d1d200ff41c64aefa8a5",
        argumentsSha256: "5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9",
        nonceSha256: "d".repeat(64), expiresAtUnixMs: Date.now() + 60_000
      })),
      begin: vi.fn(async () => undefined),
      finishToolInvocation: vi.fn(async () => undefined),
      resolveMcpToolCommand: vi.fn(async () => { throw new Error("no completed invocation"); }),
      executeMessageCommand: vi.fn(async (input: { invocationId: string }) => {
        invocationIds.push(input.invocationId);
        return { resourceType: "message" as const, resourceId: "MSG-1", commandKind: "assistant_reply" as const, commandId: `tool:${input.invocationId}` };
      })
    };
    const replyContext = { ...context, approvedCapabilities: ["message.assistant_reply.send"] };
    await createInteractiveReplyExecutor(client).execute(
      { conversationId: "direct:U100:UAI", content: "notice", eventId: "E-INT-1", occurredAtUnixMs: 1_700_000_000_000 }, replyContext
    );
    expect(client.authorizeInteractiveReply).toHaveBeenCalledOnce();
    const mintArgs = client.authorizeInteractiveReply.mock.calls[0]!;
    expect(mintArgs[0]).toBe("TASK-1");
    expect(mintArgs[1]).toBe("RUN-1");
    expect(mintArgs[2]).toMatchObject({
      capabilityId: "message.assistant_reply.send",
      resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] },
      argumentsSha256: "5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9",
      expiresAtUnixMs: 1_700_000_000_000 + 30 * 60 * 1_000
    });
    expect(mintArgs[2].approvalId).toMatch(/^approval:[0-9a-f]{48}$/);
    expect(client.executeMessageCommand).toHaveBeenCalledOnce();
    expect(client.executeMessageCommand.mock.calls[0]![0]).toMatchObject({ commandKind: "assistant_reply" });
  });

  it("reuses one message command after an uncertain Core response", async () => {
    const commandCalls: Array<{ invocationId: string }> = [];
    const persistedCommandIds = new Set<string>();
    const finish = vi.fn(async () => undefined);
    const client = {
      consumeApproval: vi.fn(async () => undefined),
      resolveApprovalGrant: vi.fn(async () => ({
        approvalId: "APR-1", capabilityId: "message.system.send",
        resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] },
        scopeSha256: "fda03fe9202766c3e59c11b4b069749a400e50041a44d1d200ff41c64aefa8a5",
        argumentsSha256: "5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9",
        nonceSha256: "d".repeat(64), expiresAtUnixMs: Date.now() + 60_000
      })),
      begin: vi.fn(async () => undefined),
      finishToolInvocation: finish,
      executeMessageCommand: vi.fn(async (input: { invocationId: string }) => {
        commandCalls.push({ invocationId: input.invocationId });
        persistedCommandIds.add(input.invocationId);
        if (commandCalls.length === 1) {
          throw Object.assign(new Error("response lost after commit"), { code: grpc.status.UNAVAILABLE });
        }
        return {
          resourceType: "message" as const, resourceId: "MSG-INTERACTIVE-RETRY-1",
          commandKind: "system_message" as const, commandId: `tool:${input.invocationId}`
        };
      })
    };
    const executor = createInteractiveMessageExecutor(client);
    const input = { conversationId: "direct:U100:UAI", content: "notice" };

    await expect(executor.execute(input, context)).rejects.toThrow("Tool invocation failed");
    await expect(executor.execute(input, context)).resolves.toContain("MSG-INTERACTIVE-RETRY-1");

    expect(commandCalls).toHaveLength(2);
    expect(commandCalls[0]!.invocationId).toBe(commandCalls[1]!.invocationId);
    expect(persistedCommandIds).toEqual(new Set([commandCalls[0]!.invocationId]));
    expect(finish).toHaveBeenCalledOnce();
    expect(finish).toHaveBeenCalledWith(expect.objectContaining({ status: "completed" }));
  });

  it("treats a subscription reply as delivered when its Tool Invocation already completed", async () => {
    const executeMessageCommand = vi.fn(async () => { throw new Error("resolve should not run"); });
    const client = {
      // The retried Activity re-mints idempotently, then resolve fails on the
      // spent single-use grant. A completed Tool Invocation is proof the reply
      // already committed, so the executor must skip instead of double-sending.
      authorizeSubscriptionMessage: vi.fn(async () => undefined),
      consumeApproval: vi.fn(async () => undefined),
      resolveApprovalGrant: vi.fn(async () => { throw new Error("MCP write Approval is unavailable"); }),
      begin: vi.fn(async () => undefined),
      finishToolInvocation: vi.fn(async () => undefined),
      resolveMcpToolCommand: vi.fn(async () => ({
        invocationId: "tool:sub-1", tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
        taskId: "TASK-1", runId: "RUN-1", profileId: "", serverId: "", toolName: "dipole_message_send",
        capabilityId: "message.system.send", arguments: {}, argumentsSha256: "a".repeat(64),
        startedAtUnixMs: 1_700_000_000_000, status: "completed" as const
      })),
      executeMessageCommand
    };

    await expect(createSubscriptionMessageExecutor(client).execute(
      { conversationId: "direct:U100:UAI", content: "notice", eventId: "E-SUB-1", occurredAtUnixMs: 1_700_000_000_000 }, context
    )).resolves.toBe(subscriptionReplyReplayMarker);

    expect(client.resolveMcpToolCommand).toHaveBeenCalledOnce();
    expect(executeMessageCommand).not.toHaveBeenCalled();
  });

  it("surfaces a genuine subscription reply failure when no completed invocation exists", async () => {
    const client = {
      authorizeSubscriptionMessage: vi.fn(async () => { throw Object.assign(new Error("denied"), { code: grpc.status.PERMISSION_DENIED }); }),
      consumeApproval: vi.fn(async () => undefined),
      resolveApprovalGrant: vi.fn(async () => { throw new Error("unused"); }),
      begin: vi.fn(async () => undefined),
      finishToolInvocation: vi.fn(async () => undefined),
      // No prior delivery: the probe reports a still-open invocation, so the
      // executor must rethrow rather than mask a real authorization failure.
      resolveMcpToolCommand: vi.fn(async () => ({
        invocationId: "tool:sub-1", tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
        taskId: "TASK-1", runId: "RUN-1", profileId: "", serverId: "", toolName: "dipole_message_send",
        capabilityId: "message.system.send", arguments: {}, argumentsSha256: "a".repeat(64),
        startedAtUnixMs: 1_700_000_000_000, status: "running" as const
      })),
      executeMessageCommand: vi.fn(async () => { throw new Error("unused"); })
    };

    await expect(createSubscriptionMessageExecutor(client).execute(
      { conversationId: "direct:U100:UAI", content: "notice", eventId: "E-SUB-1", occurredAtUnixMs: 1_700_000_000_000 }, context
    )).rejects.toThrow("denied");
  });
});

const messageInputSchema = z.object({
  conversationId: z.string().trim().min(1).max(256),
  content: z.string().trim().min(1).max(16 * 1024)
}).strict();

function writeRegistry(): CapabilityRegistry {
  const registry = new CapabilityRegistry();
  registry.register({
    descriptor: { id: "message.system.send", risk: "write", requiredPermission: "message.write", approvalRequired: true },
    inputSchema: messageInputSchema,
    resolveResource: input => ({ resourceType: "conversation", resourceId: input.conversationId, action: "write" }),
    execute: async () => { throw new Error("Message writes require an audited Tool Invocation"); }
  });
  return registry;
}
