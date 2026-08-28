import { describe, expect, it, vi } from "vitest";

import type { AgentEvent, AgentIdentity } from "../events/shadow-processor.js";
import { TemporalMcpSubscriptionRouteSelector } from "./mcp-subscription-route-selector.js";

const event: AgentEvent = {
  eventId: "EVENT-1",
  eventType: "message.direct.created",
  aggregateId: "MESSAGE-1",
  occurredAt: "2026-08-28T08:00:00.000Z",
  payload: { content: "inspect issue 42" },
  subscriptionId: "SUB-1",
  subscriptionBinding: {
    subscriptionId: "SUB-1",
    definitionId: "DEF-GUARDIAN",
    definitionVersion: 3,
    tenantId: "dipole",
    agentId: "UAI"
  }
};
const identity: AgentIdentity = {
  tenantId: "dipole",
  principalUuid: "U100",
  agentUuid: "UAI"
};

describe("TemporalMcpSubscriptionRouteSelector", () => {
  it("selects a host route by the exact admitted definition version", async () => {
    const resolveArguments = vi.fn(() => ({ owner: "dipole", repo: "server", issue_number: 42 }));
    const selector = new TemporalMcpSubscriptionRouteSelector([{
      definitionId: "DEF-GUARDIAN",
      definitionVersion: 3,
      routeId: "github-issue-read",
      resolveArguments
    }]);

    await expect(selector.select(event, identity)).resolves.toEqual({
      routeId: "github-issue-read",
      arguments: { owner: "dipole", repo: "server", issue_number: 42 }
    });
    expect(resolveArguments).toHaveBeenCalledWith(event, identity);
  });

  it("requires a complete subscription binding and rejects definition drift", async () => {
    const selector = new TemporalMcpSubscriptionRouteSelector([{
      definitionId: "DEF-GUARDIAN",
      definitionVersion: 3,
      routeId: "github-issue-read",
      resolveArguments: () => ({})
    }]);

    await expect(selector.select({ ...event, subscriptionBinding: undefined }, identity))
      .rejects.toThrow(/binding is unavailable/i);
    await expect(selector.select({
      ...event,
      subscriptionBinding: { ...event.subscriptionBinding!, definitionVersion: 4 }
    }, identity)).rejects.toThrow(/route is unavailable/i);
    await expect(selector.select({
      ...event,
      subscriptionBinding: { ...event.subscriptionBinding!, subscriptionId: "SUB-FORGED" }
    }, identity)).rejects.toThrow(/binding is invalid/i);
    await expect(selector.select({
      ...event,
      subscriptionBinding: { ...event.subscriptionBinding!, tenantId: "other" }
    }, identity)).rejects.toThrow(/binding is invalid/i);
  });

  it("rejects empty, duplicate or malformed host route definitions", () => {
    const route = {
      definitionId: "DEF-GUARDIAN",
      definitionVersion: 3,
      routeId: "github-issue-read",
      resolveArguments: () => ({})
    };

    expect(() => new TemporalMcpSubscriptionRouteSelector([])).toThrow(/routes are unavailable/i);
    expect(() => new TemporalMcpSubscriptionRouteSelector([route, route])).toThrow(/definition binding is duplicated/i);
    expect(() => new TemporalMcpSubscriptionRouteSelector([{ ...route, routeId: "forged route" }]))
      .toThrow(/route definition is invalid/i);
  });

  it("rejects invalid resolver arguments before returning a route selection", async () => {
    const selector = new TemporalMcpSubscriptionRouteSelector([{
      definitionId: "DEF-GUARDIAN",
      definitionVersion: 3,
      routeId: "github-issue-read",
      resolveArguments: () => ["not", "an", "object"]
    }]);

    await expect(selector.select(event, identity)).rejects.toThrow(/arguments are invalid/i);
  });
});
