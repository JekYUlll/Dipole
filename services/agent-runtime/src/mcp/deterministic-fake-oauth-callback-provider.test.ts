import { createHash } from "node:crypto";
import { describe, expect, it } from "vitest";

import { DeterministicFakeOAuthCallbackProvider, type DeterministicOutcome } from "./deterministic-fake-oauth-callback-provider.js";
import type { OAuthCallbackHandoffClaim } from "./oauth-callback-handoff-claim-client.js";

const handoff = Object.freeze({
  handoffId: "a".repeat(22), transactionId: "b".repeat(22), ownerUserId: "c".repeat(22),
  issuer: "https://auth.example.com", redirectUri: "https://dipole.example.com/oauth/callback",
  authorizationCodeSHA256: "d".repeat(64), sealedAuthorizationCode: "v1.n.c.t.w",
  runtimeKeyId: "runtime-key-1", expiresAt: new Date(Date.now() + 300_000), leaseExpiresAt: new Date(Date.now() + 30_000)
}) satisfies OAuthCallbackHandoffClaim;

function digest(code: string): string { return createHash("sha256").update(code).digest("hex"); }

describe("DeterministicFakeOAuthCallbackProvider", () => {
  it("returns the declared outcome for a code", async () => {
    const plan = new Map<string, DeterministicOutcome>([
      [digest("code-a"), { kind: "exchanged", tokens: { accessToken: "at", tokenType: "Bearer", expiresAt: new Date(Date.now() + 60_000) } }],
      [digest("code-b"), { kind: "retryable_failure", reason: "timeout" }],
      [digest("code-c"), { kind: "permanent_failure", reason: "invalid_grant" }]
    ]);
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan });
    await expect(provider.exchange({ authorizationCode: "code-a", handoff })).resolves.toMatchObject({ kind: "exchanged" });
    await expect(provider.exchange({ authorizationCode: "code-b", handoff })).resolves.toMatchObject({ kind: "retryable_failure" });
    await expect(provider.exchange({ authorizationCode: "code-c", handoff })).resolves.toMatchObject({ kind: "permanent_failure" });
  });

  it("is idempotent per authorization code for terminal outcomes and counts every attempt", async () => {
    const plan = new Map<string, DeterministicOutcome>([
      [digest("code-a"), { kind: "exchanged", tokens: { accessToken: "at", tokenType: "Bearer", expiresAt: new Date(Date.now() + 60_000) } }]
    ]);
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan });
    const first = await provider.exchange({ authorizationCode: "code-a", handoff });
    const second = await provider.exchange({ authorizationCode: "code-a", handoff });
    expect(first).toBe(second);
    expect(provider.exchangeCount("code-a")).toBe(2);
  });

  it("does not cache retryable failures so a replan can succeed on retry", async () => {
    const plan = new Map<string, DeterministicOutcome>([[digest("code"), { kind: "retryable_failure", reason: "timeout" }]]);
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan });
    await provider.exchange({ authorizationCode: "code", handoff });
    plan.set(digest("code"), { kind: "exchanged", tokens: { accessToken: "at", tokenType: "Bearer", expiresAt: new Date(Date.now() + 60_000) } });
    await expect(provider.exchange({ authorizationCode: "code", handoff })).resolves.toMatchObject({ kind: "exchanged" });
  });

  it("falls back to the default outcome for unplanned codes", async () => {
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan: new Map(), defaultOutcome: { kind: "retryable_failure", reason: "no plan" } });
    await expect(provider.exchange({ authorizationCode: "unknown", handoff })).resolves.toMatchObject({ kind: "retryable_failure" });
  });

  it("supports throw outcomes for ambiguous-provider simulation", async () => {
    const plan = new Map<string, DeterministicOutcome>([[digest("boom"), { kind: "throw", reason: "connection reset" }]]);
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan });
    await expect(provider.exchange({ authorizationCode: "boom", handoff })).rejects.toThrow("connection reset");
  });
});
