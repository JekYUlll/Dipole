import { describe, expect, it, vi } from "vitest";

import {
  McpToolRoundTerminalError,
  type McpToolRoundReceiptLocator
} from "./mcp-input-required-activity.js";
import { McpTerminalWorkerRuntime } from "./mcp-terminal-worker-runtime.js";
import type { McpWorkerCommandDispatcher } from "./mcp-worker-dispatch.js";

const ids = { taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1" };
const receipt: McpToolRoundReceiptLocator = { roundId: "a".repeat(64), roundNumber: 0 };

describe("MCP terminal Worker Runtime", () => {
  it("finishes a completed Invocation from only durable receipt IDs", async () => {
    const dispatcher = {
      begin: vi.fn(async () => ({ kind: "complete" as const, result: { content: [] }, receipt })),
      resume: vi.fn()
    } as unknown as McpWorkerCommandDispatcher;
    const finishMcpToolInvocationFromRound = vi.fn(async () => ({ invocationId: "INV-1", status: "completed" as const }));
    const runtime = new McpTerminalWorkerRuntime(dispatcher, { finishMcpToolInvocationFromRound });

    await expect(runtime.begin(ids)).resolves.toMatchObject({ kind: "complete", receipt });
    expect(finishMcpToolInvocationFromRound).toHaveBeenCalledWith({ ...ids, roundId: receipt.roundId });
  });

  it("finishes a failed Invocation before preserving the Activity failure", async () => {
    const terminalError = new McpToolRoundTerminalError(receipt, "remote_outcome_unknown");
    const dispatcher = {
      begin: vi.fn(async () => { throw terminalError; }),
      resume: vi.fn()
    } as unknown as McpWorkerCommandDispatcher;
    const finishMcpToolInvocationFromRound = vi.fn(async () => ({ invocationId: "INV-1", status: "failed" as const }));
    const runtime = new McpTerminalWorkerRuntime(dispatcher, { finishMcpToolInvocationFromRound });

    await expect(runtime.begin(ids)).rejects.toBe(terminalError);
    expect(finishMcpToolInvocationFromRound).toHaveBeenCalledWith({ ...ids, roundId: receipt.roundId });
  });

  it("does not finish a waiting or ambiguous Invocation", async () => {
    const dispatcher = {
      begin: vi.fn(async () => ({ kind: "wait_input" as const, checkpoint: {}, directive: {} })),
      resume: vi.fn()
    } as unknown as McpWorkerCommandDispatcher;
    const finishMcpToolInvocationFromRound = vi.fn();
    const runtime = new McpTerminalWorkerRuntime(dispatcher, { finishMcpToolInvocationFromRound });

    await expect(runtime.begin(ids)).resolves.toMatchObject({ kind: "wait_input" });
    expect(finishMcpToolInvocationFromRound).not.toHaveBeenCalled();
  });
});
