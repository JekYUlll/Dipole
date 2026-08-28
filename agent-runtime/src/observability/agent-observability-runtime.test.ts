import { describe, expect, it, vi } from "vitest";
import { createServer } from "node:http";
import { once } from "node:events";

import {
  createAgentObservabilityRuntime,
  loadAgentObservabilityConfig
} from "./agent-observability-runtime.js";
import { AgentTelemetry } from "./agent-telemetry.js";

describe("Agent observability configuration", () => {
  it("stays disabled without constructing an implicit localhost exporter", async () => {
    const factory = vi.fn();
    const runtime = createAgentObservabilityRuntime(loadAgentObservabilityConfig({
      DIPOLE_AGENT_OTEL_ENABLED: "false",
      OTEL_EXPORTER_OTLP_TRACES_PROTOCOL: "invalid-residual-value",
      OTEL_TRACES_SAMPLER_ARG: "invalid-residual-value"
    }), factory);

    runtime.start();
    await runtime.stop();

    expect(factory).not.toHaveBeenCalled();
  });

  it("uses standard OTEL variables with bounded parent-based sampling", () => {
    expect(loadAgentObservabilityConfig({
      DIPOLE_AGENT_OTEL_ENABLED: "true",
      OTEL_EXPORTER_OTLP_ENDPOINT: "https://collector.example.test/otel",
      OTEL_EXPORTER_OTLP_TRACES_PROTOCOL: "http/protobuf",
      OTEL_TRACES_SAMPLER: "parentbased_traceidratio",
      OTEL_TRACES_SAMPLER_ARG: "0.25",
      OTEL_EXPORTER_OTLP_TRACES_TIMEOUT: "4200",
      OTEL_SERVICE_NAME: "dipole-agent-shadow"
    })).toEqual({
      enabled: true,
      endpoint: "https://collector.example.test/otel/v1/traces",
      sampleRatio: 0.25,
      exportTimeoutMs: 4200,
      serviceName: "dipole-agent-shadow"
    });
  });

  it("rejects ambiguous protocols, unsafe endpoints, and invalid sample ratios", () => {
    const base = {
      DIPOLE_AGENT_OTEL_ENABLED: "true",
      OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: "https://collector.example.test/v1/traces"
    };
    expect(() => loadAgentObservabilityConfig({ ...base, OTEL_EXPORTER_OTLP_TRACES_PROTOCOL: "grpc" })).toThrow(/http\/protobuf/);
    expect(() => loadAgentObservabilityConfig({ ...base, OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: "file:///tmp/traces" })).toThrow(/HTTP/);
    expect(() => loadAgentObservabilityConfig({ ...base, OTEL_TRACES_SAMPLER_ARG: "1.1" })).toThrow(/sample ratio/);
  });
});

describe("Agent observability lifecycle", () => {
  it("starts one SDK and shuts it down once", async () => {
    const sdk = { start: vi.fn(), shutdown: vi.fn(async () => undefined) };
    const factory = vi.fn(() => sdk);
    const runtime = createAgentObservabilityRuntime(loadAgentObservabilityConfig({
      DIPOLE_AGENT_OTEL_ENABLED: "true",
      OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: "http://collector:4318/v1/traces"
    }), factory);

    runtime.start();
    expect(() => runtime.start()).toThrow(/already started/);
    await runtime.stop();
    await runtime.stop();

    expect(factory).toHaveBeenCalledOnce();
    expect(sdk.start).toHaveBeenCalledOnce();
    expect(sdk.shutdown).toHaveBeenCalledOnce();
  });

  it("flushes a real OTLP protobuf span without sensitive content", async () => {
    let requestPath = "";
    let contentType = "";
    const chunks: Buffer[] = [];
    let resolveReceived: () => void = () => undefined;
    const received = new Promise<void>(resolve => { resolveReceived = resolve; });
    const collector = createServer((request, response) => {
      requestPath = request.url ?? "";
      contentType = request.headers["content-type"] ?? "";
      request.on("data", chunk => chunks.push(Buffer.from(chunk)));
      request.on("end", () => {
        response.writeHead(200, { "content-type": "application/x-protobuf" });
        response.end();
        resolveReceived();
      });
    });
    collector.listen(0, "127.0.0.1");
    await once(collector, "listening");
    const address = collector.address();
    if (address === null || typeof address === "string") throw new Error("OTLP test collector did not bind TCP");
    const runtime = createAgentObservabilityRuntime(loadAgentObservabilityConfig({
      DIPOLE_AGENT_OTEL_ENABLED: "true",
      OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: `http://127.0.0.1:${address.port}/v1/traces`,
      OTEL_TRACES_SAMPLER_ARG: "1"
    }));

    try {
      runtime.start();
      const result = await new AgentTelemetry().withSpan("agent.test.export", { taskId: "TASK-EXPORT" }, async span => {
        span.setAttribute("dipole.agent.test.count", 1);
        return "sensitive prompt";
      });
      expect(result).toBe("sensitive prompt");
      await runtime.stop();
      await received;
    } finally {
      collector.close();
      await once(collector, "close");
    }

    const payload = Buffer.concat(chunks);
    expect(requestPath).toBe("/v1/traces");
    expect(contentType).toContain("application/x-protobuf");
    expect(payload.byteLength).toBeGreaterThan(0);
    expect(payload.includes(Buffer.from("sensitive prompt"))).toBe(false);
  });
});
