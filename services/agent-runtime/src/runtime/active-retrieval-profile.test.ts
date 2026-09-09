import { describe, expect, it } from "vitest";

import { assertActiveRetrievalProfile } from "./active-retrieval-profile.js";

const profile = {
  runtimeMode: "active" as const,
  temporal: { enabled: true, address: "temporal:7233", namespace: "dipole", taskQueue: "dipole-agent-retrieval-v1", activityMode: "retrieval_active" as const },
  capabilityRPCEnabled: true, capabilityRPCTLS: true, controlEnabled: true,
  mcpServerEnabled: false, externalMcpEnabled: false, memoryEnabled: false,
  retrievalEnabled: true, retrievalContextEnabled: true, subscriptionShadowEnabled: false,
  interactiveMessageWritesEnabled: false, subscriptionMessageWritesEnabled: false
};

describe("active retrieval Agent profile", () => {
  it("accepts the isolated read-only retrieval surface", () => {
    expect(() => assertActiveRetrievalProfile(profile)).not.toThrow();
  });

  it.each([
    ["retrievalEnabled", "requires retrieval"], ["retrievalContextEnabled", "requires retrieval"],
    ["interactiveMessageWritesEnabled", "interactive message writes"], ["memoryEnabled", "Memory"]
  ] as const)("rejects %s profile drift", (key, message) => {
    const value = key === "interactiveMessageWritesEnabled" || key === "memoryEnabled";
    expect(() => assertActiveRetrievalProfile({ ...profile, [key]: value })).toThrow(message);
  });
});
