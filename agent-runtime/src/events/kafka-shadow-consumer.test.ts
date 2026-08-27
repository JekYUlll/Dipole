import { describe, expect, it, vi } from "vitest";

import { KafkaShadowConsumer, type KafkaConsumerFactoryPort, type KafkaConsumerPort } from "./kafka-shadow-consumer.js";

describe("KafkaShadowConsumer", () => {
  it("uses an isolated group, subscribes explicitly, and awaits processing before return", async () => {
    let eachMessage: ((payload: { message: { value: Buffer | null } }) => Promise<void>) | undefined;
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
    await eachMessage!({ message: { value: Buffer.from("event") } });
    expect(process).toHaveBeenCalledWith("event");
    await expect(eachMessage!({ message: { value: null } })).rejects.toThrow(/empty Kafka message/);
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
});
