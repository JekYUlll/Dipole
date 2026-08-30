import { describe, expect, it } from "vitest";

import { assertActiveReadProfileSurface } from "./active-read-profile.js";

const readOnlySurface = {
  controlEnabled: false,
  mcpServerEnabled: false,
  externalMcpEnabled: false,
  memoryEnabled: false,
  subscriptionShadowEnabled: false
};

describe("active Agent read profile", () => {
  it("accepts the constrained active surface and leaves Shadow configurable", () => {
    expect(() => assertActiveReadProfileSurface("active", readOnlySurface)).not.toThrow();
    expect(() => assertActiveReadProfileSurface("shadow", {
      controlEnabled: true, mcpServerEnabled: true, externalMcpEnabled: true, memoryEnabled: true, subscriptionShadowEnabled: true
    })).not.toThrow();
  });

  it.each([
    ["controlEnabled", "Control API"],
    ["mcpServerEnabled", "MCP Server"],
    ["externalMcpEnabled", "External MCP"],
    ["memoryEnabled", "Memory"],
    ["subscriptionShadowEnabled", "subscription Shadow"]
  ] as const)("rejects active %s", (key, name) => {
    expect(() => assertActiveReadProfileSurface("active", { ...readOnlySurface, [key]: true })).toThrow(name);
  });
});
