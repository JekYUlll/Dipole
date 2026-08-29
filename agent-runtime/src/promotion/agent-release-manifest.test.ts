import { describe, expect, it } from "vitest";

import { agentReleaseManifestSha256, assertShadowPromotionBinding, parseAgentReleaseManifest } from "./agent-release-manifest.js";

const suiteHash = "a".repeat(64);

describe("Agent release manifest", () => {
  it("binds all upgrade components to a shadow candidate and stable hash", () => {
    const manifest = fixture();
    const parsed = parseAgentReleaseManifest(manifest);
    expect(parsed.components.capabilitySchema.sha256).toHaveLength(64);
    expect(assertShadowPromotionBinding(manifest, "agent-runtime@abc123", suiteHash)).toEqual(parsed);
    expect(agentReleaseManifestSha256(manifest)).toMatch(/^[a-f0-9]{64}$/);
    expect(agentReleaseManifestSha256({ ...manifest, stage: "shadow" })).toBe(agentReleaseManifestSha256(manifest));
  });

  it.each([
    ["offline", "Agent release manifest is not in shadow stage"],
    ["user_gray", "Agent release manifest is not in shadow stage"]
  ])("rejects promotion from %s stage", (stage, message) => {
    expect(() => assertShadowPromotionBinding({ ...fixture(), stage }, "agent-runtime@abc123", suiteHash)).toThrow(message);
  });

  it("rejects candidate, Eval Suite, and unknown field drift", () => {
    expect(() => assertShadowPromotionBinding(fixture(), "agent-runtime@other", suiteHash)).toThrow(/candidate version/);
    expect(() => assertShadowPromotionBinding(fixture(), "agent-runtime@abc123", "b".repeat(64))).toThrow(/Eval Suite hash/);
    expect(() => parseAgentReleaseManifest({ ...fixture(), extra: true })).toThrow();
  });
});

function fixture() {
  const component = (name: string) => ({ version: `${name}@1`, sha256: "b".repeat(64) });
  return {
    schemaVersion: "dipole.agent.release-manifest.v1" as const,
    candidateVersion: "agent-runtime@abc123",
    runtimeId: "dipole-agent" as const,
    stage: "shadow" as const,
    components: {
      model: component("model"), prompt: component("prompt"), capabilitySchema: component("capability"), memoryPolicy: component("memory")
    },
    offlineEvalSuiteSha256: suiteHash,
    createdAt: "2026-08-29T00:00:00.000Z"
  };
}
