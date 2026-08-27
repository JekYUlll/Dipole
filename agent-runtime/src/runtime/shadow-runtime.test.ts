import { describe, expect, it, vi } from "vitest";

import type { KafkaConsumerFactoryPort, KafkaConsumerPort } from "../events/kafka-shadow-consumer.js";
import type { AgentEvent } from "../events/shadow-processor.js";
import type { ExecutionContext } from "./execution-context.js";
import { buildKafkaShadowRuntime, loadShadowRuntimeConfig } from "./shadow-runtime.js";

describe("shadow runtime composition", () => {
  it("requires brokers only when Kafka shadow mode is enabled", () => {
    expect(loadShadowRuntimeConfig({})).toMatchObject({ enabled: false, groupId: "dipole-agent-shadow-v1" });
    expect(() => loadShadowRuntimeConfig({ DIPOLE_AGENT_KAFKA_ENABLED: "true" })).toThrow(/brokers/);
    expect(loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true",
      DIPOLE_AGENT_KAFKA_BROKERS: "kafka-1:9092, kafka-2:9092"
    }).brokers).toEqual(["kafka-1:9092", "kafka-2:9092"]);
  });

  it("decodes a Kafka envelope and records a read-only plan", async () => {
    let eachMessage: ((payload: { message: { value: Buffer | null } }) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: async () => undefined,
      subscribe: async () => undefined,
      run: async (config) => { eachMessage = config.eachMessage; },
      disconnect: async () => undefined
    };
    const factory: KafkaConsumerFactoryPort = { create: () => consumer };
    const planner = { plan: vi.fn(async (_event: AgentEvent, _context: ExecutionContext) => ({ summary: "observe", capabilityIds: [] })) };
    const audit = { append: vi.fn(async () => undefined) };
    const config = loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_UUID: "UAI"
    });
    const runtime = buildKafkaShadowRuntime(config, factory, planner, audit);
    await runtime.start();
    await eachMessage!({ message: { value: Buffer.from(JSON.stringify(messageEnvelope())) } });

    expect(planner).toHaveProperty("plan");
    expect(planner.plan.mock.calls[0]?.[1]).toMatchObject({
      principalUuid: "U100", agentUuid: "UAI", mode: "shadow", requestId: "R1", traceId: "T1", eventId: "E1"
    });
    expect(audit.append).toHaveBeenCalledWith(expect.objectContaining({ eventId: "E1", eventType: "message.direct.created" }));
  });

  it("ignores direct messages addressed to another Agent", async () => {
    let eachMessage: ((payload: { message: { value: Buffer | null } }) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: async () => undefined,
      subscribe: async () => undefined,
      run: async (config) => { eachMessage = config.eachMessage; },
      disconnect: async () => undefined
    };
    const planner = { plan: vi.fn(async () => ({ summary: "observe", capabilityIds: [] })) };
    const audit = { append: vi.fn(async () => undefined) };
    const config = loadShadowRuntimeConfig({
      DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092", DIPOLE_AGENT_UUID: "UAI"
    });
    const runtime = buildKafkaShadowRuntime(config, { create: () => consumer }, planner, audit);

    await runtime.start();
    await eachMessage!({ message: { value: Buffer.from(JSON.stringify(messageEnvelope("OTHER"))) } });

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
