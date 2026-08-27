import { createHash } from "node:crypto";
import { describe, expect, it, vi } from "vitest";
import type { Span, Tracer } from "@opentelemetry/api";

import type { ExecutionContext } from "../runtime/execution-context.js";
import { McpToolInvocationRunner } from "./mcp-tool-invocation.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1", mode: "shadow",
  permissions: ["conversation.list"], resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["list"] }],
  approvedCapabilities: [], requestId: "REQ-1", traceId: "TRACE-1"
};

describe("McpToolInvocationRunner", () => {
  it("persists hashed begin/completed evidence inside an OTel ToolCall span", async () => {
    const begin = vi.fn(async () => undefined);
    const finish = vi.fn(async () => undefined);
    const { tracer, span } = tracerFixture();
    const runner = new McpToolInvocationRunner({ begin, finish }, tracer, () => "INV-1", monotonicClock(100, 112));
    const result = await runner.execute({ name: "dipole_conversation_list", capabilityId: "conversation.list" }, { limit: 20 }, context, async () => ({ conversations: ["secret-body"] }));

    expect(result).toBe(JSON.stringify({ conversations: ["secret-body"] }));
    expect(begin).toHaveBeenCalledWith({
      invocationId: "INV-1", taskId: "TASK-1", runId: "RUN-1", toolName: "dipole_conversation_list",
      capabilityId: "conversation.list", argumentsSha256: sha(JSON.stringify({ limit: 20 })), requestId: "REQ-1", traceId: "TRACE-1"
    });
    expect(finish).toHaveBeenCalledWith({
      invocationId: "INV-1", taskId: "TASK-1", runId: "RUN-1", status: "completed",
      resultSha256: sha(result), resultBytes: Buffer.byteLength(result), latencyMs: 12
    });
    expect(span.setAttribute).toHaveBeenCalledWith("dipole.agent.tool.invocation_id", "INV-1");
    expect(span.setAttribute).not.toHaveBeenCalledWith(expect.anything(), expect.stringContaining("secret-body"));
    expect(span.end).toHaveBeenCalledOnce();
  });

  it("uses language-neutral canonical arguments independent of key order", async () => {
    const begin = vi.fn(async () => undefined);
    const finish = vi.fn(async () => undefined);
    const runner = new McpToolInvocationRunner({ begin, finish }, tracerFixture().tracer, () => "INV-CANON", monotonicClock(0, 1));
    await runner.execute(
      { name: "canonical_probe", capabilityId: "conversation.list" },
      { conversationId: "direct:U100:UAI", content: "notice" }, context, async () => ({ ok: true })
    );
    expect(begin).toHaveBeenCalledWith(expect.objectContaining({ argumentsSha256: "5ffc80e79ae2e6723a320e67256994b9954fe7b8acd0e1126a27bd5d03c50db9" }));
  });

  it("records a bounded failed terminal and withholds Tool errors", async () => {
    const begin = vi.fn(async () => undefined);
    const finish = vi.fn(async () => undefined);
    const { tracer, span } = tracerFixture();
    const runner = new McpToolInvocationRunner({ begin, finish }, tracer, () => "INV-2", monotonicClock(10, 15));
    await expect(runner.execute(
      { name: "dipole_conversation_list", capabilityId: "conversation.list" }, { limit: 20 }, context,
      async () => { throw new Error("sensitive database detail"); }
    )).rejects.toThrow("Tool invocation failed");
    expect(finish).toHaveBeenCalledWith({
      invocationId: "INV-2", taskId: "TASK-1", runId: "RUN-1", status: "failed", errorCode: "tool_execution_failed", latencyMs: 5
    });
    expect(JSON.stringify(finish.mock.calls)).not.toContain("sensitive database detail");
    expect(span.recordException).toHaveBeenCalledWith(expect.objectContaining({ message: "agent.tool.call failed" }));
    expect(JSON.stringify(span.recordException.mock.calls)).not.toContain("sensitive database detail");
    expect(span.end).toHaveBeenCalledOnce();
  });

  it("binds an approved write to its Message Command result", async () => {
    const begin = vi.fn(async () => undefined);
    const finish = vi.fn(async () => undefined);
    const runner = new McpToolInvocationRunner({ begin, finish }, tracerFixture().tracer, () => "INV-W", monotonicClock(20, 29));
    const value = { messageId: "MSG-1" };
    const result = await runner.execute(
      { name: "dipole_message_send", capabilityId: "message.system.send", approvalId: "APR-1" },
      { conversationId: "direct:U100:UAI", content: "notice" }, context, async () => value,
      output => ({ resourceType: "message", resourceId: (output as typeof value).messageId, commandKind: "system_message", commandId: "CMD-1" })
    );

    expect(begin).toHaveBeenCalledWith(expect.objectContaining({ invocationId: "INV-W", approvalId: "APR-1" }));
    expect(finish).toHaveBeenCalledWith(expect.objectContaining({
      invocationId: "INV-W", status: "completed",
      actionReference: { resourceType: "message", resourceId: "MSG-1", commandKind: "system_message", commandId: "CMD-1" }
    }));
    expect(result).toBe(JSON.stringify(value));
    expect(() => runner.execute(
      { name: "dipole_message_send", capabilityId: "message.system.send", approvalId: "APR-1" }, {}, context, async () => value
    )).toThrow(/Approval/);
  });

  it("does not execute without a durable begin and fails closed on oversized results", async () => {
    const execute = vi.fn(async () => ({ ok: true }));
    const begin = vi.fn(async () => { throw new Error("audit unavailable"); });
    const finish = vi.fn(async () => undefined);
    const runner = new McpToolInvocationRunner({ begin, finish }, tracerFixture().tracer, () => "INV-3", monotonicClock(0, 1));
    await expect(runner.execute({ name: "list", capabilityId: "conversation.list" }, {}, context, execute)).rejects.toThrow("Tool audit unavailable");
    expect(execute).not.toHaveBeenCalled();
    expect(finish).not.toHaveBeenCalled();

    const successfulBegin = vi.fn(async () => undefined);
    const oversized = new McpToolInvocationRunner({ begin: successfulBegin, finish }, tracerFixture().tracer, () => "INV-4", monotonicClock(0, 2));
    await expect(oversized.execute({ name: "list", capabilityId: "conversation.list" }, {}, context, async () => "x".repeat(65 * 1024))).rejects.toThrow("Tool invocation failed");
    expect(finish).toHaveBeenLastCalledWith(expect.objectContaining({ status: "failed", errorCode: "result_too_large" }));
  });

  it("aborts a timed-out Tool and persists one stable terminal", async () => {
    vi.useFakeTimers();
    try {
      const begin = vi.fn(async () => undefined);
      const finish = vi.fn(async () => undefined);
      const operation = vi.fn((signal: AbortSignal) => new Promise<unknown>((_resolve, reject) => {
        signal.addEventListener("abort", () => reject(signal.reason), { once: true });
      }));
      const runner = new McpToolInvocationRunner(
        { begin, finish }, tracerFixture().tracer, () => "INV-TIMEOUT", monotonicClock(0, 250), 200
      );
      const pending = runner.execute({ name: "list", capabilityId: "conversation.list" }, {}, context, operation);
      await vi.advanceTimersByTimeAsync(200);
      await expect(pending).rejects.toThrow("Tool invocation failed");
      expect(operation).toHaveBeenCalledWith(expect.objectContaining({ aborted: true }), "INV-TIMEOUT");
      expect(finish).toHaveBeenCalledOnce();
      expect(finish).toHaveBeenCalledWith(expect.objectContaining({
        invocationId: "INV-TIMEOUT", status: "failed", errorCode: "tool_timeout", latencyMs: 250
      }));
    } finally {
      vi.useRealTimers();
    }
  });

  it("rejects unbounded Tool timeout configuration", () => {
    expect(() => new McpToolInvocationRunner({ begin: vi.fn(), finish: vi.fn() }, tracerFixture().tracer, () => "INV", () => 0, 99)).toThrow(/timeout/);
    expect(() => new McpToolInvocationRunner({ begin: vi.fn(), finish: vi.fn() }, tracerFixture().tracer, () => "INV", () => 0, 60_001)).toThrow(/timeout/);
  });
});

function sha(value: string): string {
  return createHash("sha256").update(value).digest("hex");
}

function monotonicClock(...values: number[]): () => number {
  let index = 0;
  return () => values[Math.min(index++, values.length - 1)]!;
}

function tracerFixture(): { tracer: Tracer; span: { [K in keyof Span]: ReturnType<typeof vi.fn> } } {
  const span = {
    spanContext: vi.fn(), setAttribute: vi.fn(), setAttributes: vi.fn(), addEvent: vi.fn(), addLink: vi.fn(), addLinks: vi.fn(),
    setStatus: vi.fn(), updateName: vi.fn(), end: vi.fn(), isRecording: vi.fn(() => true), recordException: vi.fn()
  } as unknown as { [K in keyof Span]: ReturnType<typeof vi.fn> };
  const tracer = { startActiveSpan: vi.fn((_name: string, _options: unknown, callback: (span: Span) => unknown) => callback(span as unknown as Span)) } as unknown as Tracer;
  return { tracer, span };
}
