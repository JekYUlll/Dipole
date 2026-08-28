import { describe, expect, it, vi } from "vitest";

import {
  ExternalMcpActivityRoundSessionFactory,
  McpInputRequiredActivity,
  type McpActivityModernClient,
  type McpActivityRoundSession,
  type McpActivityRoundSessionFactory,
  type McpToolRoundReceiptClient
} from "./mcp-input-required-activity.js";

describe("MCP input-required Activity boundary", () => {
  it("opens and closes a fresh session around each durable round", async () => {
    const sessions: McpActivityRoundSession[] = [];
    const close = vi.fn(async () => undefined);
    const factory: McpActivityRoundSessionFactory = {
      open: vi.fn(async () => {
        const index = sessions.length;
        const session: McpActivityRoundSession = {
          callToolRound: vi.fn(async () => index === 0 ? inputRequired() : ({ content: [{ type: "text" as const, text: "created" }] })),
          close
        };
        sessions.push(session);
        return session;
      })
    };
    const receiptClient = receipts();
    const activity = new McpInputRequiredActivity(factory, receiptClient, () => 1_000);

    const wait = await activity.begin(command());
    expect(wait.kind).toBe("wait_input");
    if (wait.kind !== "wait_input") throw new Error("expected durable wait");
    expect(wait.directive).toMatchObject({ kind: "wait_input", requestId: "INPUT-1" });
    expect(wait.checkpoint).toMatchObject({
      schemaVersion: "dipole.mcp.input-required-activity-checkpoint.v1",
      tenantId: "dipole", profileId: "calendar-prod", serverId: "calendar.example"
    });
    expect(JSON.stringify(wait.checkpoint)).not.toMatch(/password|token|credential/i);
    expect(close).toHaveBeenCalledTimes(1);

    const completed = await new McpInputRequiredActivity(factory, receiptClient, () => 1_100).resume(wait.checkpoint, {
      action: "accept", resume: {
        kind: "input", requestId: "INPUT-1", value: { title: "Review", visibility: "team" }
      }
    });
    expect(completed).toEqual({ kind: "complete", result: { content: [{ type: "text", text: "created" }] } });
    expect(factory.open).toHaveBeenCalledTimes(2);
    expect(close).toHaveBeenCalledTimes(2);
    expect(receiptClient.finishMcpToolRound).toHaveBeenCalledTimes(2);
    expect(sessions[1]!.callToolRound).toHaveBeenCalledWith({
      name: "calendar.create",
      arguments: { calendarId: "CAL-1" },
      inputResponses: {
        "event-settings": { action: "accept", content: { title: "Review", visibility: "team" } }
      },
      requestState: "opaque-state-1"
    }, undefined);
  });

  it("closes the session after cancellation or Tool failure", async () => {
    const close = vi.fn(async () => undefined);
    const controller = new AbortController();
    const factory: McpActivityRoundSessionFactory = {
      open: async () => {
        controller.abort();
        return {
        callToolRound: async () => { throw new Error("request_cancelled"); },
        close
        };
      }
    };

    await expect(new McpInputRequiredActivity(factory, receipts()).begin(command(), controller.signal))
      .rejects.toThrow(/aborted/);
    expect(close).toHaveBeenCalledOnce();
  });

  it("rejects outer binding drift and a second input_required round", async () => {
    let calls = 0;
    const factory: McpActivityRoundSessionFactory = {
      open: async () => ({
        callToolRound: async () => { calls += 1; return inputRequired(); },
        close: async () => undefined
      })
    };
    const activity = new McpInputRequiredActivity(factory, receipts(), () => 1_000);
    const wait = await activity.begin(command());
    if (wait.kind !== "wait_input") throw new Error("expected durable wait");

    await expect(activity.resume({ ...wait.checkpoint, profileId: "other-profile" }, {
      action: "cancel", requestId: "INPUT-1"
    })).rejects.toThrow(/integrity/i);
    await expect(activity.resume(wait.checkpoint, {
      action: "cancel", requestId: "INPUT-1"
    })).rejects.toThrow(/additional input_required/i);
    expect(calls).toBe(2);
  });

  it("replays a completed receipt without opening a session and fails closed on ambiguity", async () => {
    const factory: McpActivityRoundSessionFactory = { open: vi.fn() };
    const completed = receipts({
      outcome: "replay_completed", result: { content: [{ type: "text", text: "replayed" }] },
      resultJSON: `{"content":[{"text":"replayed","type":"text"}]}`, resultSha256: "a".repeat(64)
    });
    await expect(new McpInputRequiredActivity(factory, completed).begin(command())).resolves.toEqual({
      kind: "complete", result: { content: [{ type: "text", text: "replayed" }] }
    });
    expect(factory.open).not.toHaveBeenCalled();
    expect(completed.finishMcpToolRound).not.toHaveBeenCalled();

    await expect(new McpInputRequiredActivity(factory, receipts({ outcome: "ambiguous" })).begin(command()))
      .rejects.toThrow(/automatic retry is disabled/i);
    expect(factory.open).not.toHaveBeenCalled();
  });

  it("creates a modern allowlisted Client from tenant-owned profile metadata", async () => {
    const transport = {} as never;
    const registry = {
      describe: vi.fn(() => ({
        profileId: "calendar-prod", tenantId: "dipole", serverId: "calendar.example",
        allowedTools: ["calendar.create"]
      })),
      connect: vi.fn(async () => transport)
    };
    const connect = vi.fn(async () => []);
    const client: McpActivityModernClient = {
      connect,
      callToolRound: vi.fn(async () => ({ content: [{ type: "text" as const, text: "created" }] })),
      close: vi.fn(async () => undefined)
    };
    const createClient = vi.fn(() => client);
    const factory = new ExternalMcpActivityRoundSessionFactory(registry, {
      "calendar-prod": {
        "calendar.create": { allowedArgumentNames: ["calendarId"], maximumBytes: 1024 }
      }
    }, 5_000, createClient);

    await expect(factory.open({
      tenantId: "dipole", profileId: "calendar-prod", serverId: "calendar.example", toolName: "calendar.create"
    })).resolves.toBe(client);
    expect(createClient).toHaveBeenCalledWith(expect.objectContaining({
      protocolMode: "modern", serverId: "calendar.example", allowedTools: ["calendar.create"]
    }));
    expect(connect).toHaveBeenCalledWith(transport);
    expect(registry.connect).toHaveBeenCalledWith("calendar-prod", "dipole", undefined);

    await expect(factory.open({
      tenantId: "dipole", profileId: "calendar-prod", serverId: "forged.example", toolName: "calendar.create"
    })).rejects.toThrow(/Server binding/i);
    expect(registry.connect).toHaveBeenCalledTimes(1);
  });

  it("closes a fresh Client when transport setup is cancelled or handshake fails", async () => {
    const controller = new AbortController();
    const close = vi.fn(async () => undefined);
    const closeTransport = vi.fn(async () => undefined);
    const transport = { close: closeTransport } as never;
    const client: McpActivityModernClient = {
      connect: vi.fn(async () => { throw new Error("handshake failed"); }),
      callToolRound: vi.fn(),
      close
    };
    const profile = {
      profileId: "calendar-prod", tenantId: "dipole", serverId: "calendar.example",
      allowedTools: ["calendar.create"]
    };
    const registry = {
      describe: () => profile,
      connect: vi.fn(async () => transport)
    };
    const factory = new ExternalMcpActivityRoundSessionFactory(registry, {
      "calendar-prod": { "calendar.create": { allowedArgumentNames: ["calendarId"], maximumBytes: 1024 } }
    }, 5_000, () => client);

    await expect(factory.open({
      tenantId: "dipole", profileId: "calendar-prod", serverId: "calendar.example", toolName: "calendar.create"
    })).rejects.toThrow(/handshake failed/);
    expect(close).toHaveBeenCalledOnce();
    expect(closeTransport).toHaveBeenCalledOnce();

    registry.connect.mockImplementationOnce(async () => {
      controller.abort();
      return transport;
    });
    await expect(factory.open({
      tenantId: "dipole", profileId: "calendar-prod", serverId: "calendar.example", toolName: "calendar.create"
    }, controller.signal)).rejects.toThrow(/aborted/);
    expect(close).toHaveBeenCalledTimes(2);
    expect(closeTransport).toHaveBeenCalledTimes(2);
  });
});

function command() {
  return {
    requestId: "INPUT-1",
    taskId: "TASK-1",
    runId: "RUN-1",
    tenantId: "dipole",
    profileId: "calendar-prod",
    serverId: "calendar.example",
    toolName: "calendar.create",
    invocationId: "INV-1",
    arguments: { calendarId: "CAL-1" },
    expiresAtUnixMs: 2_000
  };
}

function receipts(
  outcome: Awaited<ReturnType<McpToolRoundReceiptClient["claimMcpToolRound"]>> = { outcome: "claimed" }
): McpToolRoundReceiptClient {
  return {
    claimMcpToolRound: vi.fn(async () => outcome),
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
          message: "Choose event settings",
          requestedSchema: {
            type: "object" as const,
            properties: {
              title: { type: "string" as const, title: "Event title", maxLength: 120 },
              visibility: { type: "string" as const, title: "Visibility", enum: ["team", "private"] }
            },
            required: ["title", "visibility"]
          }
        }
      }
    },
    requestState: "opaque-state-1"
  };
}
