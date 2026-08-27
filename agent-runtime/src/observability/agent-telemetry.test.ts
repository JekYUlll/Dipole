import { describe, expect, it, vi } from "vitest";
import { SpanStatusCode, type Span, type Tracer } from "@opentelemetry/api";

import { AgentTelemetry } from "./agent-telemetry.js";

describe("AgentTelemetry", () => {
  it("records bounded identity and numeric result attributes", async () => {
    const fixture = tracerFixture();
    const telemetry = new AgentTelemetry(fixture.tracer);

    await expect(telemetry.withSpan("agent.model.call", {
      taskId: "TASK-1", runId: "RUN-1", attributes: { "dipole.agent.model.route": "gateway/primary" }
    }, async span => {
      span.setAttribute("dipole.agent.model.input_tokens", 12);
      return "ok";
    })).resolves.toBe("ok");

    expect(fixture.tracer.startActiveSpan).toHaveBeenCalledWith("agent.model.call", {}, expect.any(Function));
    expect(fixture.span.setAttributes).toHaveBeenCalledWith({
      "dipole.agent.task.id": "TASK-1", "dipole.agent.run.id": "RUN-1",
      "dipole.agent.model.route": "gateway/primary"
    });
    expect(fixture.span.setStatus).toHaveBeenCalledWith({ code: SpanStatusCode.OK });
    expect(fixture.span.end).toHaveBeenCalledOnce();
  });

  it("records a stable failure class without leaking exception text", async () => {
    const fixture = tracerFixture();
    const telemetry = new AgentTelemetry(fixture.tracer);

    await expect(telemetry.withSpan("agent.artifact.create", { taskId: "TASK-1", runId: "RUN-1" }, async () => {
      throw new Error("secret object storage credential");
    })).rejects.toThrow("secret object storage credential");

    expect(fixture.span.recordException).toHaveBeenCalledWith(expect.objectContaining({ message: "agent.artifact.create failed" }));
    expect(JSON.stringify(fixture.span.recordException.mock.calls)).not.toContain("credential");
    expect(fixture.span.setStatus).toHaveBeenCalledWith({ code: SpanStatusCode.ERROR });
    expect(fixture.span.end).toHaveBeenCalledOnce();
  });
});

function tracerFixture(): { tracer: Tracer; span: { [K in keyof Span]: ReturnType<typeof vi.fn> } } {
  const span = {
    spanContext: vi.fn(), setAttribute: vi.fn(), setAttributes: vi.fn(), addEvent: vi.fn(), addLink: vi.fn(), addLinks: vi.fn(),
    setStatus: vi.fn(), updateName: vi.fn(), end: vi.fn(), isRecording: vi.fn(() => true), recordException: vi.fn()
  } as unknown as { [K in keyof Span]: ReturnType<typeof vi.fn> };
  const tracer = {
    startActiveSpan: vi.fn((_name: string, _options: unknown, callback: (span: Span) => unknown) => callback(span as unknown as Span))
  } as unknown as Tracer;
  return { tracer, span };
}
