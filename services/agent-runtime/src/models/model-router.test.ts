import { describe, expect, it, vi } from "vitest";
import { SpanStatusCode, type Span, type Tracer } from "@opentelemetry/api";
import { z } from "zod";

import { ModelRouter, ModelRoutingError, type ModelAuditStore, type ModelTimelineSink, type StructuredModelClient } from "./model-router.js";
import { AgentTelemetry } from "../observability/agent-telemetry.js";

const outputSchema = z.object({ summary: z.string() });

describe("ModelRouter", () => {
  it("uses the primary route and applies the per-call budget", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn(async () => ({
      output: { summary: "primary" }, usage: { inputTokens: 12, outputTokens: 4 }
    }));
    const router = new ModelRouter({ generate }, ["gateway/fast", "gateway/fallback"], {
      maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 128
    });

    const result = await router.generate({ prompt: "plan event", schema: outputSchema });

    expect(result).toEqual({
      output: { summary: "primary" }, route: "gateway/fast", attempts: 1,
      usage: { inputTokens: 12, outputTokens: 4 }
    });
    expect(generate).toHaveBeenCalledWith(expect.objectContaining({
      route: "gateway/fast", maxOutputTokens: 128, timeoutMs: expect.any(Number)
    }));
  });

  it("falls back in order and charges failed calls to the run", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn()
      .mockRejectedValueOnce(new Error("primary unavailable"))
      .mockResolvedValueOnce({ output: { summary: "fallback" }, usage: { inputTokens: 8, outputTokens: 3 } });
    const router = new ModelRouter({ generate }, ["gateway/primary", "gateway/fallback"], {
      maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    });

    const result = await router.generate({ prompt: "plan event", schema: outputSchema });

    expect(result.route).toBe("gateway/fallback");
    expect(result.attempts).toBe(2);
    expect(generate).toHaveBeenCalledTimes(2);
  });

  it("falls back when a provider returns schema-incompatible output", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn()
      .mockResolvedValueOnce({ output: { summary: 42 }, usage: { inputTokens: 8, outputTokens: 3 } })
      .mockResolvedValueOnce({ output: { summary: "validated" }, usage: { inputTokens: 9, outputTokens: 4 } });
    const router = new ModelRouter({ generate }, ["primary", "fallback"], {
      maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    });

    await expect(router.generate({ prompt: "plan event", schema: outputSchema })).resolves.toMatchObject({
      output: { summary: "validated" }, route: "fallback", attempts: 2
    });
  });

  it("stops before an unbudgeted fallback", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn(async () => { throw new Error("unavailable"); });
    const router = new ModelRouter({ generate }, ["one", "two", "three"], {
      maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    });

    const failure = await router.generate({ prompt: "plan event", schema: outputSchema }).catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(ModelRoutingError);
    expect(failure).toMatchObject({ attempts: 2, exhaustedBudget: true });
    expect(generate).toHaveBeenCalledTimes(2);
  });

  it("does not start another route after the run deadline", async () => {
    let now = 1000;
    const generate: StructuredModelClient["generate"] = vi.fn(async () => {
      now = 1200;
      throw new Error("timeout");
    });
    const router = new ModelRouter({ generate }, ["one", "two"], {
      maxCalls: 2, totalTimeoutMs: 100, maxOutputTokensPerCall: 64
    }, () => now);

    const failure = await router.generate({ prompt: "plan event", schema: outputSchema }).catch((error: unknown) => error);

    expect(failure).toMatchObject({ attempts: 1, exhaustedBudget: true });
    expect(generate).toHaveBeenCalledTimes(1);
    expect(generate).toHaveBeenCalledWith(expect.objectContaining({ timeoutMs: 100 }));
  });

  it("persists the call and Run lifecycle around a successful provider call", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn(async () => ({
      output: { summary: "persisted" }, usage: { inputTokens: 12, outputTokens: 4 }, finishReason: "stop"
    }));
    const audit = auditStore();
    vi.mocked(audit.reserve).mockResolvedValue({ runId: "RUN-1", callId: "CALL-1", callNo: 1, route: "primary" });
    const router = new ModelRouter({ generate }, ["primary"], {
      maxCalls: 1, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    }, () => 1000, audit);

    await expect(router.generate({ prompt: "plan", schema: outputSchema, taskId: "TASK-1" })).resolves.toMatchObject({
      output: { summary: "persisted" }, route: "primary"
    });
    expect(audit.completeCall).toHaveBeenCalledWith(
      expect.objectContaining({ callId: "CALL-1" }), { summary: "persisted" },
      { inputTokens: 12, outputTokens: 4 }, "stop", 0
    );
    expect(audit.completeRun).toHaveBeenCalledWith("RUN-1");
    expect(audit.failCall).not.toHaveBeenCalled();
  });

  it("replays a validated completed call without invoking the provider again", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn();
    const audit = auditStore();
    vi.mocked(audit.recover).mockResolvedValue({
      runId: "RUN-1", callId: "CALL-1", callNo: 1, route: "primary",
      output: { summary: "recovered" }, usage: { inputTokens: 12, outputTokens: 4 }
    });
    const router = new ModelRouter({ generate }, ["primary"], {
      maxCalls: 1, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    }, () => 1000, audit);

    await expect(router.generate({ prompt: "plan", schema: outputSchema, taskId: "TASK-1" })).resolves.toEqual({
      output: { summary: "recovered" }, route: "primary", attempts: 1,
      usage: { inputTokens: 12, outputTokens: 4 }
    });
    expect(generate).not.toHaveBeenCalled();
    expect(audit.recover).toHaveBeenCalledWith("TASK-1", {
      maxCalls: 1, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    });
    expect(audit.reserve).not.toHaveBeenCalled();
    expect(audit.completeRun).toHaveBeenCalledWith("RUN-1");
  });

  it("projects model call lifecycle without exposing model output", async () => {
    const timeline: ModelTimelineSink = { appendAgentTaskTimelineEvent: vi.fn(async () => undefined) };
    const audit = auditStore();
    vi.mocked(audit.reserve).mockResolvedValue({ runId: "RUN-1", callId: "CALL-1", callNo: 1, route: "primary" });
    const router = new ModelRouter({ generate: vi.fn(async () => ({ output: { summary: "ok" }, usage: { inputTokens: 1, outputTokens: 2 } })) }, ["primary"], {
      maxCalls: 1, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    }, undefined, audit, undefined, timeline);
    await router.generate({ prompt: "plan", schema: outputSchema, taskId: "TASK-1" });
    expect(timeline.appendAgentTaskTimelineEvent).toHaveBeenNthCalledWith(1, expect.objectContaining({
      eventId: "model:CALL-1:begin", taskId: "TASK-1", runId: "RUN-1", kind: "model_call", status: "running"
    }));
    expect(timeline.appendAgentTaskTimelineEvent).toHaveBeenNthCalledWith(2, expect.objectContaining({
      eventId: "model:CALL-1:finish", status: "completed"
    }));
    for (const [event] of vi.mocked(timeline.appendAgentTaskTimelineEvent).mock.calls) {
      expect(event).not.toHaveProperty("output");
      expect(event).not.toHaveProperty("prompt");
    }
  });

  it("records provider failure before reserving the fallback", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn()
      .mockRejectedValueOnce(new Error("primary failed"))
      .mockResolvedValueOnce({ output: { summary: "fallback" }, usage: { inputTokens: 8, outputTokens: 3 }, finishReason: "stop" });
    const audit = auditStore();
    vi.mocked(audit.reserve)
      .mockResolvedValueOnce({ runId: "RUN-1", callId: "CALL-1", callNo: 1, route: "primary" })
      .mockResolvedValueOnce({ runId: "RUN-1", callId: "CALL-2", callNo: 2, route: "fallback" });
    const router = new ModelRouter({ generate }, ["primary", "fallback"], {
      maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    }, () => 1000, audit);

    await router.generate({ prompt: "plan", schema: outputSchema, taskId: "TASK-1" });

    expect(audit.failCall).toHaveBeenCalledWith(expect.objectContaining({ callId: "CALL-1" }), expect.any(Error), 0);
    expect(audit.completeCall).toHaveBeenCalledWith(
      expect.objectContaining({ callId: "CALL-2" }), { summary: "fallback" },
      { inputTokens: 8, outputTokens: 3 }, "stop", 0
    );
  });

  it("does not call a provider when the durable Task budget has no slot", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn();
    const audit = auditStore();
    vi.mocked(audit.reserve).mockResolvedValue(undefined);
    const router = new ModelRouter({ generate }, ["primary"], {
      maxCalls: 1, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    }, () => 1000, audit);

    await expect(router.generate({ prompt: "plan", schema: outputSchema, taskId: "TASK-1" })).rejects.toMatchObject({
      attempts: 0, exhaustedBudget: true
    });
    expect(generate).not.toHaveBeenCalled();
    expect(audit.failTask).toHaveBeenCalledWith("TASK-1", expect.any(ModelRoutingError));
  });

  it("records one ModelCall span per provider attempt", async () => {
    const spanAttributes: Array<Record<string, unknown>> = [];
    const tracer = {
      startActiveSpan: vi.fn((_name: string, _options: unknown, callback: (span: Span) => unknown) => {
        const span = {
          setAttributes: vi.fn((attributes: Record<string, unknown>) => { spanAttributes.push(attributes); return span; }),
          setAttribute: vi.fn((key: string, value: unknown) => { spanAttributes.push({ [key]: value }); return span; }),
          setStatus: vi.fn(), recordException: vi.fn(), end: vi.fn()
        } as unknown as Span;
        return callback(span);
      })
    } as unknown as Tracer;
    const generate: StructuredModelClient["generate"] = vi.fn()
      .mockRejectedValueOnce(new Error("primary unavailable"))
      .mockResolvedValueOnce({ output: { summary: "fallback" }, usage: { inputTokens: 8, outputTokens: 3 } });
    const router = new ModelRouter(
      { generate }, ["primary", "fallback"],
      { maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64 },
      () => 1000, undefined, new AgentTelemetry(tracer)
    );

    await router.generate({ prompt: "sensitive prompt", schema: outputSchema, taskId: "TASK-1" });

    expect(tracer.startActiveSpan).toHaveBeenCalledTimes(2);
    expect(tracer.startActiveSpan).toHaveBeenNthCalledWith(1, "agent.model.call", {}, expect.any(Function));
    expect(spanAttributes).toEqual(expect.arrayContaining([
      expect.objectContaining({ "dipole.agent.model.route": "primary", "dipole.agent.model.attempt": 1 }),
      expect.objectContaining({ "dipole.agent.model.route": "fallback", "dipole.agent.model.attempt": 2 }),
      expect.objectContaining({ "dipole.agent.model.input_tokens": 8 })
    ]));
    expect(JSON.stringify(spanAttributes)).not.toContain("sensitive prompt");
  });

  it("marks schema-incompatible provider output as a failed ModelCall", async () => {
    const statuses: unknown[] = [];
    const tracer = {
      startActiveSpan: vi.fn((_name: string, _options: unknown, callback: (span: Span) => unknown) => callback({
        setAttributes: vi.fn(), setAttribute: vi.fn(),
        setStatus: vi.fn(status => statuses.push(status)), recordException: vi.fn(), end: vi.fn()
      } as unknown as Span))
    } as unknown as Tracer;
    const generate: StructuredModelClient["generate"] = vi.fn()
      .mockResolvedValueOnce({ output: { summary: 42 }, usage: { inputTokens: 8, outputTokens: 3 } })
      .mockResolvedValueOnce({ output: { summary: "validated" }, usage: { inputTokens: 9, outputTokens: 4 } });
    const router = new ModelRouter(
      { generate }, ["primary", "fallback"],
      { maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64 },
      () => 1000, undefined, new AgentTelemetry(tracer)
    );

    await router.generate({ prompt: "plan", schema: outputSchema, taskId: "TASK-1" });

    expect(statuses).toEqual([{ code: SpanStatusCode.ERROR }, { code: SpanStatusCode.OK }]);
  });
});

function auditStore(): ModelAuditStore {
  return {
    recover: vi.fn(async () => undefined), reserve: vi.fn(), completeCall: vi.fn(async () => undefined), failCall: vi.fn(async () => undefined),
    completeRun: vi.fn(async () => undefined), failRun: vi.fn(async () => undefined), failTask: vi.fn(async () => undefined)
  };
}
