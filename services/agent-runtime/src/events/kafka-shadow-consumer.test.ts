import { describe, expect, it, vi } from "vitest";

import { KafkaFailureRouter, type KafkaFailureMessage } from "./kafka-failure-router.js";
import { KafkaShadowConsumer, type KafkaConsumerFactoryPort, type KafkaConsumerPort, type KafkaInboundPayload } from "./kafka-shadow-consumer.js";

describe("KafkaShadowConsumer", () => {
  it("uses an isolated group, subscribes explicitly, and awaits processing before return", async () => {
    let eachMessage: ((payload: KafkaInboundPayload) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: vi.fn(async () => undefined),
      subscribe: vi.fn(async () => undefined),
      run: vi.fn(async (config) => { eachMessage = config.eachMessage; }),
      disconnect: vi.fn(async () => undefined)
    };
    const factory: KafkaConsumerFactoryPort = { create: vi.fn(() => consumer) };
    const process = vi.fn(async () => undefined);
    const runtime = new KafkaShadowConsumer(factory, {
      groupId: "dipole-agent-shadow-v1",
      topic: "message.direct.created"
    }, process);

    await runtime.start();
    expect(factory.create).toHaveBeenCalledWith("dipole-agent-shadow-v1");
    expect(consumer.connect).toHaveBeenCalledOnce();
    expect(consumer.subscribe).toHaveBeenCalledWith({ topic: "message.direct.created", fromBeginning: false });
    expect(eachMessage).toBeDefined();
    await eachMessage!(payload(Buffer.from("event")));
    expect(process).toHaveBeenCalledWith("event");
    await expect(eachMessage!(payload(null))).rejects.toThrow(/empty Kafka message/);
    await runtime.stop();
    expect(consumer.disconnect).toHaveBeenCalledOnce();
  });

  it("rejects a group shared with the Embedded runtime", () => {
    const factory = {} as KafkaConsumerFactoryPort;
    expect(() => new KafkaShadowConsumer(factory, {
      groupId: "dipole-core-consumer",
      topic: "message.direct.created"
    }, async () => undefined)).toThrow(/isolated/);
  });

  it("accepts an isolated active group and rejects unrelated groups in active mode", () => {
    const factory = {} as KafkaConsumerFactoryPort;
    expect(() => new KafkaShadowConsumer(factory, {
      groupId: "dipole-agent-active-user-gray-v1",
      topic: "message.direct.created",
      runtimeMode: "active"
    }, async () => undefined)).not.toThrow();
    expect(() => new KafkaShadowConsumer(factory, {
      groupId: "dipole-agent-shadow-v1",
      topic: "message.direct.created",
      runtimeMode: "active"
    }, async () => undefined)).toThrow(/active.*dipole-agent-active/i);
    expect(() => new KafkaShadowConsumer(factory, {
      groupId: "dipole-agent-subscription-active-v1",
      topic: "message.direct.created",
      runtimeMode: "active"
    }, async () => undefined)).toThrow(/dipole-agent-active/i);
  });

  it("accepts the subscription Active group only when the subscription surface is enabled", () => {
    const factory = {} as KafkaConsumerFactoryPort;
    expect(() => new KafkaShadowConsumer(factory, {
      groupId: "dipole-agent-subscription-active-v1",
      topic: "message.direct.created",
      runtimeMode: "active",
      subscriptionActiveEnabled: true
    }, async () => undefined)).not.toThrow();
  });

  it("recreates and disconnects the consumer when cold-start metadata is not ready", async () => {
    const first: KafkaConsumerPort = {
      connect: vi.fn(async () => undefined),
      subscribe: vi.fn(async () => undefined),
      run: vi.fn(async () => { throw new Error("topic metadata not ready"); }),
      disconnect: vi.fn(async () => undefined)
    };
    const second: KafkaConsumerPort = {
      connect: vi.fn(async () => undefined),
      subscribe: vi.fn(async () => undefined),
      run: vi.fn(async () => undefined),
      disconnect: vi.fn(async () => undefined)
    };
    const factory: KafkaConsumerFactoryPort = {
      create: vi.fn()
        .mockReturnValueOnce(first)
        .mockReturnValueOnce(second)
    };
    const runtime = new KafkaShadowConsumer(factory, {
      groupId: "dipole-agent-shadow-v1",
      topic: "message.direct.created",
      startupAttempts: 2,
      startupRetryDelayMs: 0
    }, async () => undefined);

    await runtime.start();
    expect(first.disconnect).toHaveBeenCalledOnce();
    expect(second.run).toHaveBeenCalledOnce();
    await runtime.stop();
    expect(second.disconnect).toHaveBeenCalledOnce();
  });

  it("disconnects the final failed consumer before surfacing startup failure", async () => {
    const consumer: KafkaConsumerPort = {
      connect: vi.fn(async () => undefined),
      subscribe: vi.fn(async () => undefined),
      run: vi.fn(async () => { throw new Error("broker unavailable"); }),
      disconnect: vi.fn(async () => undefined)
    };
    const runtime = new KafkaShadowConsumer({ create: () => consumer }, {
      groupId: "dipole-agent-shadow-v1",
      topic: "message.direct.created",
      startupAttempts: 1,
      startupRetryDelayMs: 0
    }, async () => undefined);

    await expect(runtime.start()).rejects.toThrow(/broker unavailable/);
    expect(consumer.disconnect).toHaveBeenCalledOnce();
  });

  it("subscribes to retry and transfers failed processing with original metadata", async () => {
    let eachMessage: ((payload: KafkaInboundPayload) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: async () => undefined,
      subscribe: vi.fn(async () => undefined),
      run: async (config) => { eachMessage = config.eachMessage; },
      disconnect: async () => undefined
    };
    const publish = vi.fn(async (_topic: string, _message: Omit<KafkaFailureMessage, "topic">) => undefined);
    const runtime = new KafkaShadowConsumer({ create: () => consumer }, {
      groupId: "dipole-agent-shadow-v1", topic: "dipole.message.direct.created"
    }, async () => { throw new Error("planner unavailable"); }, new KafkaFailureRouter({ publish }, 3));

    await runtime.start();
    expect(consumer.subscribe).toHaveBeenNthCalledWith(2, {
      topic: "dipole.message.direct.created.retry", fromBeginning: false
    });
    await eachMessage!({
      topic: "dipole.message.direct.created",
      message: { key: Buffer.from("M1"), value: Buffer.from("event"), headers: { event_id: Buffer.from("E1") } }
    });
    expect(publish).toHaveBeenCalledWith("dipole.message.direct.created.retry", expect.objectContaining({
      key: Buffer.from("M1"), value: Buffer.from("event"), headers: expect.objectContaining({ event_id: "E1", retry_attempt: "1" })
    }));
  });

  it("routes tombstones directly to dead letter", async () => {
    let eachMessage: ((payload: KafkaInboundPayload) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: async () => undefined,
      subscribe: async () => undefined,
      run: async (config) => { eachMessage = config.eachMessage; },
      disconnect: async () => undefined
    };
    const publish = vi.fn(async (_topic: string, _message: Omit<KafkaFailureMessage, "topic">) => undefined);
    const process = vi.fn(async () => undefined);
    const runtime = new KafkaShadowConsumer({ create: () => consumer }, {
      groupId: "dipole-agent-shadow-v1", topic: "dipole.message.direct.created"
    }, process, new KafkaFailureRouter({ publish }, 3));

    await runtime.start();
    await eachMessage!({
      topic: "dipole.message.direct.created",
      message: { key: Buffer.from("M1"), value: null }
    });

    expect(process).not.toHaveBeenCalled();
    expect(publish).toHaveBeenCalledWith("dipole.message.direct.created.dead", expect.objectContaining({
      value: null,
      headers: expect.objectContaining({ dead_reason: "invalid_envelope", retry_attempt: "0" })
    }));
  });

  it("rejects the handler when failure routing cannot publish", async () => {
    let eachMessage: ((payload: KafkaInboundPayload) => Promise<void>) | undefined;
    const consumer: KafkaConsumerPort = {
      connect: async () => undefined,
      subscribe: async () => undefined,
      run: async (config) => { eachMessage = config.eachMessage; },
      disconnect: async () => undefined
    };
    const runtime = new KafkaShadowConsumer({ create: () => consumer }, {
      groupId: "dipole-agent-shadow-v1", topic: "dipole.message.direct.created"
    }, async () => { throw new Error("planner unavailable"); }, new KafkaFailureRouter({
      publish: async () => { throw new Error("failure publisher unavailable"); }
    }, 3));

    await runtime.start();
    await expect(eachMessage!(payload(Buffer.from("event")))).rejects.toThrow(/failure publisher unavailable/);
  });
});

function payload(value: Buffer | null): KafkaInboundPayload {
  return { topic: "message.direct.created", message: { key: Buffer.from("M1"), value } };
}
