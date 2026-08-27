import { describe, expect, it, vi } from "vitest";

import { KafkaFailureRouter, type KafkaFailureMessage, type KafkaFailurePublisher } from "./kafka-failure-router.js";

const message = {
  topic: "message.direct.created",
  key: Buffer.from("M1"),
  value: Buffer.from("event"),
  headers: { event_id: "E1", schema_version: "v1" }
};

describe("KafkaFailureRouter", () => {
  it("sends permanent envelope failures directly to dead letter with diagnostics", async () => {
    const publish = vi.fn(async (_topic: string, _message: Omit<KafkaFailureMessage, "topic">) => undefined);
    const router = new KafkaFailureRouter({ publish }, 3, () => new Date("2026-08-27T03:30:00.000Z"));

    await router.route(message, new SyntaxError("invalid JSON"), true);

    expect(publish).toHaveBeenCalledWith("message.direct.created.dead", expect.objectContaining({
      key: message.key,
      value: message.value,
      headers: expect.objectContaining({
        event_id: "E1", dead_reason: "invalid_envelope", last_error: "invalid JSON",
        original_topic: "message.direct.created", retry_attempt: "0", failed_at: "2026-08-27T03:30:00.000Z"
      })
    }));
  });

  it("routes transient failures through retry and then dead at the configured bound", async () => {
    const publish = vi.fn(async (_topic: string, _message: Omit<KafkaFailureMessage, "topic">) => undefined);
    const router = new KafkaFailureRouter({ publish }, 3);

    await router.route(message, new Error("temporary"), false);
    await router.route({ ...message, topic: "message.direct.created.retry", headers: { ...message.headers, retry_attempt: "1" } }, new Error("again"), false);
    await router.route({ ...message, topic: "message.direct.created.retry", headers: { ...message.headers, retry_attempt: "2" } }, new Error("final"), false);

    expect(publish.mock.calls.map(([topic]) => topic)).toEqual([
      "message.direct.created.retry", "message.direct.created.retry", "message.direct.created.dead"
    ]);
    expect(publish.mock.calls[0]?.[1].headers.retry_attempt).toBe("1");
    expect(publish.mock.calls[1]?.[1].headers.retry_attempt).toBe("2");
    expect(publish.mock.calls[2]?.[1].headers.dead_reason).toBe("handler_failed");
  });

  it("surfaces publisher failure so the source offset stays uncommitted", async () => {
    const publisher: KafkaFailurePublisher = { publish: vi.fn(async () => { throw new Error("broker unavailable"); }) };
    const router = new KafkaFailureRouter(publisher, 3);

    await expect(router.route(message, new Error("planner failed"), false)).rejects.toThrow(/broker unavailable/);
  });

  it("normalizes malformed retry headers and preserves the original topic", async () => {
    const publish = vi.fn(async (_topic: string, _message: Omit<KafkaFailureMessage, "topic">) => undefined);
    const router = new KafkaFailureRouter({ publish }, 2);

    await router.route({
      ...message,
      topic: "message.direct.created.retry",
      headers: { retry_attempt: "invalid", original_topic: "message.direct.created" }
    }, "failure", false);

    expect(publish).toHaveBeenCalledWith("message.direct.created.retry", expect.objectContaining({
      headers: expect.objectContaining({ retry_attempt: "1", original_topic: "message.direct.created" })
    }));
  });
});
