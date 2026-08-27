import { describe, expect, it } from "vitest";

import { matchEventSubscriptions } from "./event-subscription.js";
import type { AgentEvent } from "./shadow-processor.js";

const event: AgentEvent = {
  eventId: "E1", eventType: "message.direct.created", aggregateId: "M1",
  occurredAt: "2026-08-27T08:00:00.000Z",
  payload: { conversation_key: "group:G1", content: "数据库迁移可能延期" }
};

describe("matchEventSubscriptions", () => {
  it("selects the lexicographically first matching subscription without a model call", () => {
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
});

function subscription(subscriptionId: string, filterKind: string, filter: unknown) {
  return {
    subscriptionId, definitionId: "DEF-1", definitionVersion: 1,
    tenantId: "dipole", agentId: "UAI", eventType: "message.direct.created",
    resourceType: "conversation", resourceId: "group:G1", filterKind, filter
  };
}
