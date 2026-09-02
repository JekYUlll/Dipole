import { describe, expect, it } from "vitest";

import { matchEventSubscriptions } from "./event-subscription.js";
import type { AgentEvent } from "./shadow-processor.js";

const event: AgentEvent = {
  eventId: "E1", eventType: "message.direct.created", aggregateId: "M1",
  occurredAt: "2026-08-27T08:00:00.000Z",
  payload: { conversation_key: "group:G1", content: "数据库迁移可能延期" }
};

describe("matchEventSubscriptions", () => {
  it("returns every matching subscription in deterministic order", () => {
    const result = matchEventSubscriptions(event, [
      subscription("SUB-B", "message_contains_any", { terms: ["延期"] }),
      subscription("SUB-A", "all", {})
    ]);

    expect(result.map((item) => item.subscriptionId)).toEqual(["SUB-A", "SUB-B"]);
  });

  it("rejects event, resource, and keyword misses before task admission", () => {
    expect(matchEventSubscriptions(event, [subscription("SUB-1", "message_contains_any", { terms: ["事故"] })])).toEqual([]);
    expect(matchEventSubscriptions(event, [{ ...subscription("SUB-2", "all", {}), resourceId: "group:G2" }])).toEqual([]);
    expect(matchEventSubscriptions(event, [{ ...subscription("SUB-3", "all", {}), eventType: "file.created" }])).toEqual([]);
  });

  it("fails closed for malformed persisted filter policy", () => {
    expect(() => matchEventSubscriptions(event, [subscription("SUB-1", "message_contains_any", { terms: [] })])).toThrow(/terms/);
    expect(() => matchEventSubscriptions(event, [subscription("SUB-2", "model", { prompt: "classify" })])).toThrow(/filterKind/);
  });

  it("bounds the candidate set before parsing and matching", () => {
    const candidates = Array.from({ length: 257 }, (_, index) => subscription(`SUB-${index}`, "all", {}));
    expect(() => matchEventSubscriptions(event, candidates)).toThrow(/candidate set exceeds 256/);
  });
});

function subscription(subscriptionId: string, filterKind: string, filter: unknown) {
  return {
    subscriptionId, definitionId: "DEF-1", definitionVersion: 1,
    tenantId: "dipole", agentId: "UAI", createdById: "U100", eventType: "message.direct.created",
    resourceType: "conversation", resourceId: "group:G1", filterKind, filter
  };
}
