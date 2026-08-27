import { describe, expect, it, vi } from "vitest";

import type { KafkaConsumerFactoryPort, KafkaConsumerPort, KafkaInboundPayload } from "../events/kafka-shadow-consumer.js";
import type { AgentEvent } from "../events/shadow-processor.js";
import type { ExecutionContext } from "./execution-context.js";
import { buildKafkaShadowRuntime, loadShadowRuntimeConfig } from "./shadow-runtime.js";

describe("shadow runtime composition", () => {
  it("requires brokers only when Kafka shadow mode is enabled", () => {
    expect(loadShadowRuntimeConfig({})).toMatchObject({
      enabled: false, groupId: "dipole-agent-shadow-v1", ledgerMode: "memory", modelMode: "metadata",
      capabilityRpc: { enabled: false }
    });
    expect(() => loadShadowRuntimeConfig({ DIPOLE_AGENT_KAFKA_ENABLED: "true" })).toThrow(/brokers/);
    expect(loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true",
      DIPOLE_AGENT_KAFKA_BROKERS: "kafka-1:9092, kafka-2:9092"
    }).brokers).toEqual(["kafka-1:9092", "kafka-2:9092"]);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_LEDGER_MODE: "mysql"
    })).toThrow(/MySQL/);
    expect(loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_LEDGER_MODE: "mysql",
      DIPOLE_AGENT_MYSQL_HOST: "mysql", DIPOLE_AGENT_MYSQL_USER: "agent", DIPOLE_AGENT_MYSQL_PASSWORD: "secret",
      DIPOLE_AGENT_MYSQL_DATABASE: "dipole"
    })).toMatchObject({ ledgerMode: "mysql", mysql: { host: "mysql", port: 3306, user: "agent", database: "dipole" } });
    expect(() => loadShadowRuntimeConfig({ DIPOLE_AGENT_MODEL_MODE: "ai_sdk" })).toThrow(/model routes/);
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_MODEL_MODE: "ai_sdk", DIPOLE_AGENT_MODEL_ROUTES: "provider/model"
    })).toThrow(/persistent MySQL model audit/);
    expect(loadShadowRuntimeConfig({
      DIPOLE_AGENT_MODEL_MODE: "ai_sdk",
      DIPOLE_AGENT_MODEL_ROUTES: "openai/gpt-5-mini,anthropic/claude-sonnet-4.5",
      DIPOLE_AGENT_LEDGER_MODE: "mysql",
      DIPOLE_AGENT_MYSQL_HOST: "mysql", DIPOLE_AGENT_MYSQL_USER: "agent", DIPOLE_AGENT_MYSQL_PASSWORD: "secret",
      DIPOLE_AGENT_MYSQL_DATABASE: "dipole",
      DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true",
      DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "127.0.0.1:9091",
      DIPOLE_INTERNAL_RPC_SHARED_SECRET: "rpc-secret",
      DIPOLE_AGENT_MODEL_MAX_CALLS: "2",
      DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS: "12000",
      DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS: "256"
    })).toMatchObject({
      modelMode: "ai_sdk",
      modelRoutes: ["openai/gpt-5-mini", "anthropic/claude-sonnet-4.5"],
      modelBudget: { maxCalls: 2, totalTimeoutMs: 12000, maxOutputTokensPerCall: 256 }
    });
    expect(() => loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092",
      DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true", DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "core:9091",
      DIPOLE_INTERNAL_RPC_SHARED_SECRET: "rpc-secret"
    })).toThrow(/loopback/);
  });

  it("decodes a Kafka envelope and records a read-only plan", async () => {
    let eachMessage: ((payload: KafkaInboundPayload) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: async () => undefined,
      subscribe: async () => undefined,
      run: async (config) => { eachMessage = config.eachMessage; },
      disconnect: async () => undefined
    };
    const factory: KafkaConsumerFactoryPort = { create: () => consumer };
    const planner = { plan: vi.fn(async (_event: AgentEvent, _context: ExecutionContext) => ({ summary: "observe", steps: [] })) };
    const audit = { append: vi.fn(async () => undefined) };
    const config = loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_UUID: "UAI"
    });
    const runtime = buildKafkaShadowRuntime(config, factory, planner, audit);
    await runtime.start();
    await eachMessage!(payload(messageEnvelope()));

    expect(planner).toHaveProperty("plan");
    expect(planner.plan.mock.calls[0]?.[1]).toMatchObject({
      principalUuid: "U100", agentUuid: "UAI", mode: "shadow", requestId: "R1", traceId: "T1", eventId: "E1"
    });
    expect(audit.append).toHaveBeenCalledWith(expect.objectContaining({ eventId: "E1", eventType: "message.direct.created" }));
  });

  it("ignores direct messages addressed to another Agent", async () => {
    let eachMessage: ((payload: KafkaInboundPayload) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: async () => undefined,
      subscribe: async () => undefined,
      run: async (config) => { eachMessage = config.eachMessage; },
      disconnect: async () => undefined
    };
    const planner = { plan: vi.fn(async () => ({ summary: "observe", steps: [] })) };
    const audit = { append: vi.fn(async () => undefined) };
    const config = loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_UUID: "UAI"
    });
    const runtime = buildKafkaShadowRuntime(config, { create: () => consumer }, planner, audit);

    await runtime.start();
    await eachMessage!(payload(messageEnvelope("OTHER")));

    expect(planner.plan).not.toHaveBeenCalled();
    expect(audit.append).not.toHaveBeenCalled();
  });
});

function messageEnvelope(targetUuid = "UAI"): object {
  return {
    event_id: "E1", request_id: "R1", trace_id: "T1",
    event_type: "message.direct.created", version: "v1", source: "dipole",
    occurred_at: "2026-08-27T08:00:00.000Z",
    payload: {
      mutation_type: "created", revision: 1, actor_uuid: "U100", message_id: "M100",
      conversation_key: "direct:U100:UAI", message_seq: 1, sender_uuid: "U100", target_uuid: targetUuid,
      target_type: 0, message_type: 0, content: "hello", sent_at: "2026-08-27T08:00:00.000Z"
    }
  };
}

function payload(envelope: object): KafkaInboundPayload {
  return {
    topic: "message.direct.created",
    message: { key: Buffer.from("M100"), value: Buffer.from(JSON.stringify(envelope)) }
  };
}
