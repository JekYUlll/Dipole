import type { Transport } from "@modelcontextprotocol/client";
import { describe, expect, it, vi } from "vitest";

import type { AgentMcpToolCommand } from "../capabilities/agent-capability-rpc.js";
import type {
  McpActivityExternalTransportRegistry,
  McpActivityModernClient,
  McpToolRoundReceiptClient
} from "./mcp-input-required-activity.js";
import {
  createMcpTerminalWorkerRuntime,
  createMcpWorkerRuntime,
  type McpWorkerCoreClient
} from "./mcp-worker-runtime.js";

const command: AgentMcpToolCommand = {
  invocationId: "INV-1",
  tenantId: "dipole",
  principalUserId: "USER-1",
  agentId: "AGENT-1",
  taskId: "TASK-1",
  runId: "RUN-1",
  profileId: "calendar-prod",
  serverId: "calendar.example",
  toolName: "calendar.create",
  capabilityId: "calendar.create",
  arguments: { calendarId: "CAL-1" },
  argumentsSha256: "aa96ead2e2c5f4724afe29d4a10200eb5a8b013adfca2c7f6caf2369028df08f",
  startedAtUnixMs: 1_000,
  status: "running"
};

describe("MCP Worker Runtime composition", () => {
  it("offers an explicit default-off terminal composition over the durable round receipt", async () => {
    const receipts = receiptStore();
    const finishMcpToolInvocationFromRound = vi.fn(async () => ({ invocationId: "INV-1", status: "completed" as const }));
    const runtime = createMcpTerminalWorkerRuntime({
      core: { ...receipts.core, finishMcpToolInvocationFromRound },
      transports: transportRegistry(vi.fn(async () => ({ close: vi.fn() }) as unknown as Transport)),
      egressPolicies: policies(),
      createClient: vi.fn(() => ({
        connect: vi.fn(async () => []),
        callToolRound: vi.fn(async () => ({ content: [] })),
        close: vi.fn(async () => undefined)
      })),
      now: () => 1_100,
      ownerTokenSha256: () => "a".repeat(64)
    });

    await expect(runtime.begin(ids())).resolves.toMatchObject({ kind: "complete", receipt: { roundNumber: 0 } });
    expect(finishMcpToolInvocationFromRound).toHaveBeenCalledWith({
      taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1", roundId: expect.stringMatching(/^[a-f0-9]{64}$/)
    });
  });

  it("replays a locally completed round after Worker replacement without reconnecting", async () => {
    const receipts = receiptStore();
    const connectTransport = vi.fn<McpActivityExternalTransportRegistry["connect"]>(
      async () => ({ close: vi.fn() }) as unknown as Transport
    );
    const callToolRound = vi.fn(async () => ({ content: [{ type: "text" as const, text: "created" }] }));
    const createClient = vi.fn((): McpActivityModernClient => ({
      connect: vi.fn(async () => []),
      callToolRound,
      close: vi.fn(async () => undefined)
    }));
    const dependencies = {
      core: receipts.core,
      transports: transportRegistry(connectTransport),
      egressPolicies: policies(),
      createClient,
      now: () => 1_100,
      ownerTokenSha256: () => "a".repeat(64)
    };

    const first = createMcpWorkerRuntime(dependencies);
    await expect(first.begin(ids())).resolves.toMatchObject({
      kind: "complete",
      result: { content: [{ type: "text", text: "created" }] },
      receipt: { roundNumber: 0 }
    });
    expect(connectTransport).toHaveBeenCalledOnce();
    expect(receipts.finish).toHaveBeenCalledOnce();

    receipts.setStatus("completed");
    const replacement = createMcpWorkerRuntime({
      ...dependencies,
      ownerTokenSha256: () => "b".repeat(64)
    });
    await expect(replacement.begin(ids())).resolves.toMatchObject({
      kind: "complete",
      result: { content: [{ type: "text", text: "created" }] },
      receipt: { roundNumber: 0 }
    });
    expect(connectTransport).toHaveBeenCalledOnce();
    expect(callToolRound).toHaveBeenCalledOnce();
  });

  it("stops an ambiguous round before creating a Client or Transport", async () => {
    const connectTransport = vi.fn<McpActivityExternalTransportRegistry["connect"]>();
    const createClient = vi.fn();
    const core = coreWithClaim({ outcome: "ambiguous" });
    const runtime = createMcpWorkerRuntime({
      core,
      transports: transportRegistry(connectTransport),
      egressPolicies: policies(),
      createClient,
      now: () => 1_100
    });

    await expect(runtime.begin(ids())).rejects.toThrow(/automatic retry is disabled/i);
    expect(createClient).not.toHaveBeenCalled();
    expect(connectTransport).not.toHaveBeenCalled();
    expect(core.finishMcpToolRound).not.toHaveBeenCalled();
  });

  it("honors pre-dispatch cancellation before Core resolution or receipt claim", async () => {
    const core = coreWithClaim({ outcome: "claimed" });
    const connectTransport = vi.fn<McpActivityExternalTransportRegistry["connect"]>();
    const controller = new AbortController();
    controller.abort(new Error("cancelled before dispatch"));
    const runtime = createMcpWorkerRuntime({
      core,
      transports: transportRegistry(connectTransport),
      egressPolicies: policies(),
      createClient: vi.fn(),
      now: () => 1_100
    });

    await expect(runtime.begin(ids(), controller.signal)).rejects.toThrow(/cancelled before dispatch/i);
    expect(core.resolveMcpToolCommand).not.toHaveBeenCalled();
    expect(core.claimMcpToolRound).not.toHaveBeenCalled();
    expect(connectTransport).not.toHaveBeenCalled();
  });
});

function ids() {
  return { taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1" };
}

function policies() {
  return {
    "calendar-prod": {
      "calendar.create": { allowedArgumentNames: ["calendarId"], maximumBytes: 1024 }
    }
  };
}

function transportRegistry(connect: McpActivityExternalTransportRegistry["connect"]): McpActivityExternalTransportRegistry {
  return {
    describe: () => ({
      profileId: "calendar-prod",
      tenantId: "dipole",
      serverId: "calendar.example",
      allowedTools: ["calendar.create"]
    }),
    connect
  };
}

function coreWithClaim(
  outcome: Awaited<ReturnType<McpToolRoundReceiptClient["claimMcpToolRound"]>>
): McpWorkerCoreClient & {
  resolveMcpToolCommand: ReturnType<typeof vi.fn>;
  claimMcpToolRound: ReturnType<typeof vi.fn>;
  finishMcpToolRound: ReturnType<typeof vi.fn>;
} {
  return {
    resolveMcpToolCommand: vi.fn(async () => command),
    claimMcpToolRound: vi.fn(async () => outcome),
    finishMcpToolRound: vi.fn(async () => undefined)
  };
}

function receiptStore() {
  let status: AgentMcpToolCommand["status"] = "running";
  let completed: {
    result: unknown;
    resultJSON: string;
    resultSha256: string;
  } | undefined;
  const finish = vi.fn(async (input: Parameters<McpToolRoundReceiptClient["finishMcpToolRound"]>[0]) => {
    if (input.status === "completed") {
      completed = {
        result: JSON.parse(input.resultJSON) as unknown,
        resultJSON: input.resultJSON,
        resultSha256: input.resultSha256
      };
    }
  });
  const core: McpWorkerCoreClient = {
    resolveMcpToolCommand: vi.fn(async () => ({ ...command, status })),
    claimMcpToolRound: vi.fn(async () => completed === undefined
      ? { outcome: "claimed" as const }
      : { outcome: "replay_completed" as const, ...completed }),
    finishMcpToolRound: finish
  };
  return { core, finish, setStatus: (value: AgentMcpToolCommand["status"]) => { status = value; } };
}
