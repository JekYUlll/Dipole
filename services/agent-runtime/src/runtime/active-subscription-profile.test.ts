import { describe, expect, it } from "vitest";

import { assertActiveSubscriptionProfile } from "./active-subscription-profile.js";

const profile = {
  runtimeMode: "active" as const,
  temporal: { enabled: true, address: "temporal:7233", namespace: "dipole", taskQueue: "dipole-agent-subscription-v1", activityMode: "subscription_active" as const },
  triggerMode: "subscription" as const,
  subscriptionActiveEnabled: true,
  capabilityRPCEnabled: true,
  capabilityRPCTLS: true,
  interactiveMessageWritesEnabled: false,
  controlEnabled: false,
  mcpServerEnabled: false,
  externalMcpEnabled: false,
  memoryEnabled: false,
  retrievalEnabled: false,
  retrievalContextEnabled: false,
  subscriptionShadowEnabled: false
};

describe("active subscription Agent profile", () => {
  it("accepts the explicit read-only subscription surface", () => {
    expect(() => assertActiveSubscriptionProfile(profile)).not.toThrow();
  });

  it.each([
    ["triggerMode", "direct_target", /subscription trigger/i],
    ["subscriptionActiveEnabled", false, /explicit subscription/i],
    ["temporal", { ...profile.temporal, taskQueue: "dipole-agent-task-v1" }, /isolated subscription task queue/i],
    ["capabilityRPCTLS", false, /RPC/i],
    ["interactiveMessageWritesEnabled", true, /message writes/i],
    ["controlEnabled", true, /Control API/i]
  ] as const)("rejects invalid %s", (key, value, expected) => {
    expect(() => assertActiveSubscriptionProfile({ ...profile, [key]: value })).toThrow(expected);
  });
});
