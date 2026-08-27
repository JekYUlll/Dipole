import { describe, expect, it } from "vitest";

import { decodeMessageCreatedEvent } from "./message-event.js";

const envelope = {
  event_id: "E1",
  request_id: "R1",
  trace_id: "T1",
  event_type: "message.direct.created",
  version: "v1.2",
  source: "dipole",
  occurred_at: "2026-08-27T08:00:00.000Z",
  payload: {
    mutation_type: "created",
    revision: 1,
    actor_uuid: "U100",
    message_id: "M100",
    conversation_key: "direct:U100:UAI",
    message_seq: 3,
    sender_uuid: "U100",
    target_uuid: "UAI",
    target_type: 0,
    message_type: 0,
    content: "hello",
    sent_at: "2026-08-27T08:00:00.000Z"
  }
};

describe("decodeMessageCreatedEvent", () => {
  it("accepts additive v1 minor fields and derives trusted trigger identity", () => {
    expect(decodeMessageCreatedEvent(JSON.stringify({ ...envelope, future_field: true }))).toEqual({
      event: {
        eventId: "E1", eventType: "message.direct.created", aggregateId: "M100",
        occurredAt: "2026-08-27T08:00:00.000Z", payload: envelope.payload
      },
      principalUuid: "U100",
      targetUuid: "UAI",
      requestId: "R1",
      traceId: "T1"
    });
  });

  it.each([
    [{ ...envelope, version: "v2" }, "version"],
    [{ ...envelope, event_type: "message.group.created" }, "event_type"],
    [{ ...envelope, source: "foreign" }, "source"],
    [{ ...envelope, payload: { ...envelope.payload, target_type: 1 } }, "target_type"],
    [{ ...envelope, payload: { ...envelope.payload, sender_uuid: "" } }, "sender_uuid"]
  ])("fails closed for incompatible envelope %#", (input, reason) => {
    expect(() => decodeMessageCreatedEvent(JSON.stringify(input))).toThrow(reason);
  });
});
