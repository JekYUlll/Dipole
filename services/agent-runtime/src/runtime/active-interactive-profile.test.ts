import { describe, expect, it } from "vitest";

import { assertActiveInteractiveProfile } from "./active-interactive-profile.js";

const profile = {
  runtimeMode: "active" as const,
  temporal: { enabled: true, address: "temporal:7233", namespace: "dipole", taskQueue: "dipole-agent-interactive-v1", activityMode: "interactive_active" as const },
  capabilityRPCEnabled: true, capabilityRPCTLS: true, interactiveMessageWritesEnabled: true,
  controlEnabled: true, mcpServerEnabled: false, externalMcpEnabled: false, memoryEnabled: false,
  retrievalEnabled: false, retrievalContextEnabled: false, subscriptionShadowEnabled: false
};

describe("active interactive Agent profile", () => {
  it("accepts only the explicit Control and message write surface", () => {
    expect(() => assertActiveInteractiveProfile(profile)).not.toThrow();
  });

  it.each([
    ["controlEnabled", "Control API"], ["interactiveMessageWritesEnabled", "message write"],
    ["mcpServerEnabled", "MCP Server"], ["memoryEnabled", "Memory"]
  ] as const)("rejects %s drift", (key, name) => {
    expect(() => assertActiveInteractiveProfile({ ...profile, [key]: key === "mcpServerEnabled" || key === "memoryEnabled" })).toThrow(name);
  });
});
