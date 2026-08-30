import { describe, expect, it } from "vitest";

import { assertActiveMemoryPromotionProfile } from "./active-memory-promotion-profile.js";

const profile = {
  runtimeMode: "active" as const,
  temporal: { enabled: true, address: "temporal:7233", namespace: "dipole", taskQueue: "dipole-agent-memory-promotion-v1", activityMode: "promotion_active" as const },
  capabilityRPCEnabled: true,
  capabilityRPCTLS: true,
  commitEnabled: true,
  authority: "operator_approved",
  controlEnabled: false,
  mcpServerEnabled: false,
  externalMcpEnabled: false,
  memoryEnabled: false,
  subscriptionShadowEnabled: false
};

describe("active Memory promotion profile", () => {
  it("accepts the narrowly scoped approved profile", () => {
    expect(() => assertActiveMemoryPromotionProfile(profile)).not.toThrow();
  });

  it.each([
    ["runtimeMode", "shadow", /active Agent Runtime/i],
    ["capabilityRPCTLS", false, /RPC mTLS/i],
    ["authority", "", /operator-approved/i],
    ["controlEnabled", true, /Control API/i]
  ] as const)("rejects invalid %s", (key, value, expected) => {
    expect(() => assertActiveMemoryPromotionProfile({ ...profile, [key]: value })).toThrow(expected);
  });

  it("rejects the promotion Activity mode without its explicit commit switch", () => {
    expect(() => assertActiveMemoryPromotionProfile({ ...profile, commitEnabled: false })).toThrow(/requires the Memory promotion commit switch/i);
  });
});
