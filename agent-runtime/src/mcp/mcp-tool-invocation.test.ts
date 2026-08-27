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
