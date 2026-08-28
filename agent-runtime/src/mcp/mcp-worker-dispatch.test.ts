import { createHash } from "node:crypto";
import { describe, expect, it, vi } from "vitest";

import type { AgentMcpToolCommand } from "../capabilities/agent-capability-rpc.js";
import {
  McpInputRequiredActivity,
  type McpActivityRoundSessionFactory,
  type McpToolRoundReceiptClient
} from "./mcp-input-required-activity.js";
import { McpWorkerCommandDispatcher } from "./mcp-worker-dispatch.js";

describe("MCP Worker command dispatch", () => {
  it("resolves an authoritative persisted command from only Task, Run, and Invocation IDs", async () => {
    const command = externalCommand();
    const resolver = { resolveMcpToolCommand: vi.fn(async () => command) };
    const open = vi.fn(async () => ({
      callToolRound: vi.fn(async () => inputRequired()),
      close: vi.fn(async () => undefined)
    }));
    const receipts = receiptClient();
    const dispatcher = new McpWorkerCommandDispatcher(
      resolver,
      new McpInputRequiredActivity({ open }, receipts, () => 1_100, () => "c".repeat(64)),
      () => 1_100
    );

    const result = await dispatcher.begin({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1" });
    expect(result.kind).toBe("wait_input");
    if (result.kind !== "wait_input") throw new Error("expected wait_input");
    expect(resolver.resolveMcpToolCommand).toHaveBeenCalledWith("TASK-1", "RUN-1", "INV-1");
    expect(open).toHaveBeenCalledWith({
      tenantId: "dipole", profileId: "calendar-prod", serverId: "calendar.example", toolName: "calendar.create"
    }, undefined);
    expect(result.checkpoint).toMatchObject({
      schemaVersion: "dipole.mcp.worker-dispatch-checkpoint.v1", taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1"
    });
    expect(result.directive.expiresAtUnixMs).toBe(901_000);
    expect(result.directive.requestId).toMatch(/^[a-f0-9]{64}$/);
    expect(receipts.finishMcpToolRound).toHaveBeenCalledOnce();
  });

  it("rejects caller-supplied authority and command drift before resumed network access", async () => {
    let command = externalCommand();
    const resolver = { resolveMcpToolCommand: vi.fn(async () => command) };
    const sessions: McpActivityRoundSessionFactory = {
      open: vi.fn(async () => ({
        callToolRound: vi.fn(async () => inputRequired()),
        close: vi.fn(async () => undefined)
      }))
    };
    const dispatcher = new McpWorkerCommandDispatcher(
      resolver,
      new McpInputRequiredActivity(sessions, receiptClient(), () => 1_100, () => "c".repeat(64)),
      () => 1_100
    );
    await expect(dispatcher.begin({
      taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1", profileId: "forged"
    })).rejects.toThrow(/dispatch input/i);
    expect(resolver.resolveMcpToolCommand).not.toHaveBeenCalled();

    const wait = await dispatcher.begin({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1" });
    if (wait.kind !== "wait_input") throw new Error("expected wait_input");
    command = { ...command, status: "completed" };
    await expect(dispatcher.resume(wait.checkpoint, {
      action: "cancel", requestId: "INPUT-1"
    })).rejects.toThrow(/command binding/i);
    expect(sessions.open).toHaveBeenCalledTimes(1);
  });

  it("re-resolves the persisted command after process replacement before resuming", async () => {
    const resolver = { resolveMcpToolCommand: vi.fn(async () => externalCommand()) };
    let calls = 0;
    const sessions: McpActivityRoundSessionFactory = {
      open: vi.fn(async () => ({
        callToolRound: vi.fn(async () => {
          calls += 1;
          return calls === 1 ? inputRequired() : { content: [{ type: "text" as const, text: "created" }] };
        }),
        close: vi.fn(async () => undefined)
      }))
    };
    const first = new McpWorkerCommandDispatcher(
      resolver,
      new McpInputRequiredActivity(sessions, receiptClient(), () => 1_100, () => "c".repeat(64)),
      () => 1_100
    );
    const wait = await first.begin({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1" });
    if (wait.kind !== "wait_input") throw new Error("expected wait_input");

    const replacement = new McpWorkerCommandDispatcher(
      resolver,
      new McpInputRequiredActivity(sessions, receiptClient(), () => 1_200, () => "d".repeat(64)),
      () => 1_200
    );
    await expect(replacement.resume(wait.checkpoint, {
      action: "accept",
      resume: { kind: "input", requestId: wait.directive.requestId, value: { title: "Review" } }
    })).resolves.toEqual({ kind: "complete", result: { content: [{ type: "text", text: "created" }] } });
    expect(resolver.resolveMcpToolCommand).toHaveBeenCalledTimes(2);
    expect(sessions.open).toHaveBeenCalledTimes(2);
  });

  it("stops cancellation during Core resolution before claiming a Tool round", async () => {
    const controller = new AbortController();
    const resolver = {
      resolveMcpToolCommand: vi.fn(async () => {
        controller.abort(new Error("cancelled during resolution"));
        return externalCommand();
      })
    };
    const receipts = receiptClient();
    const dispatcher = new McpWorkerCommandDispatcher(
      resolver,
      new McpInputRequiredActivity({ open: vi.fn() }, receipts),
      () => 1_100
    );

    await expect(dispatcher.begin({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1" }, controller.signal))
      .rejects.toThrow(/cancelled during resolution/i);
    expect(receipts.claimMcpToolRound).not.toHaveBeenCalled();
  });
});

function externalCommand(): AgentMcpToolCommand {
  const argumentsValue = { calendarId: "CAL-1" };
  const argumentsJSON = `{"calendarId":"CAL-1"}`;
  return {
    invocationId: "INV-1", tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
    taskId: "TASK-1", runId: "RUN-1", profileId: "calendar-prod", serverId: "calendar.example",
    toolName: "calendar.create", capabilityId: "conversation.list", arguments: argumentsValue,
    argumentsSha256: createHash("sha256").update(argumentsJSON).digest("hex"), startedAtUnixMs: 1_000, status: "running"
  };
}

function receiptClient(): McpToolRoundReceiptClient {
  return {
    claimMcpToolRound: vi.fn(async () => ({ outcome: "claimed" as const })),
    finishMcpToolRound: vi.fn(async () => undefined)
  };
}

function inputRequired() {
  return {
    resultType: "input_required" as const,
    inputRequests: {
      "event-settings": {
        method: "elicitation/create" as const,
        params: {
          mode: "form" as const,
          message: "Confirm event settings",
          requestedSchema: {
            type: "object" as const,
            properties: { title: { type: "string" as const, title: "Title" } },
            required: ["title"]
          }
        }
      }
    },
    requestState: "opaque-state-1"
  };
}
