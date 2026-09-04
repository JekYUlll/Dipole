import { describe, expect, it } from "vitest";

import { assertActiveReadProfileSurface } from "./active-read-profile.js";

const readOnlySurface = {
  controlEnabled: true,
  mcpServerEnabled: false,
  externalMcpEnabled: false,
  memoryEnabled: false,
  retrievalEnabled: false,
  retrievalContextEnabled: false,
  subscriptionShadowEnabled: false
};

describe("active Agent read profile", () => {
  it("accepts the constrained active surface and leaves Shadow configurable", () => {
    expect(() => assertActiveReadProfileSurface("active", readOnlySurface)).not.toThrow();
    expect(() => assertActiveReadProfileSurface("shadow", {
      controlEnabled: true, mcpServerEnabled: true, externalMcpEnabled: true, memoryEnabled: true,
      retrievalEnabled: true, retrievalContextEnabled: true, subscriptionShadowEnabled: true
    })).not.toThrow();
  });

  it.each([
    ["mcpServerEnabled", "MCP Server"],
    ["externalMcpEnabled", "External MCP"],
    ["memoryEnabled", "Memory"],
    ["retrievalEnabled", "retrieval"],
    ["retrievalContextEnabled", "retrieval Context"],
    ["subscriptionShadowEnabled", "subscription Shadow"]
  ] as const)("rejects active %s", (key, name) => {
    expect(() => assertActiveReadProfileSurface("active", { ...readOnlySurface, [key]: true })).toThrow(name);
  });
});
