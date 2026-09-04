import { createHash } from "node:crypto";
import { describe, expect, it } from "vitest";

import { DeterministicFakeOAuthCallbackProvider } from "./deterministic-fake-oauth-callback-provider.js";
import { OAuthCallbackHandoffProviderProcessor } from "./oauth-callback-handoff-provider-processor.js";
import type { OAuthCallbackHandoffClaim } from "./oauth-callback-handoff-claim-client.js";
import { TokenLifecycleStore } from "./oauth-callback-token-lifecycle.js";

const handoff = Object.freeze({
  handoffId: "a".repeat(22), transactionId: "b".repeat(22), ownerUserId: "c".repeat(22),
  issuer: "https://auth.example.com", redirectUri: "https://dipole.example.com/oauth/callback",
  authorizationCodeSHA256: "d".repeat(64), sealedAuthorizationCode: "v1.n.c.t.w",
  runtimeKeyId: "runtime-key-1", expiresAt: new Date(Date.now() + 300_000), leaseExpiresAt: new Date(Date.now() + 30_000)
}) satisfies OAuthCallbackHandoffClaim;

function digest(code: string): string { return createHash("sha256").update(code).digest("hex"); }
const tokens = { accessToken: "at", tokenType: "Bearer", expiresAt: new Date(Date.now() + 60_000) };

describe("OAuthCallbackHandoffProviderProcessor", () => {
  it("writes an active lifecycle record and reports completed on exchanged", async () => {
    const lifecycle = new TokenLifecycleStore();
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan: new Map([[digest("code"), { kind: "exchanged", tokens }]]) });
    const processor = new OAuthCallbackHandoffProviderProcessor({ provider, lifecycle });
    await expect(processor.process({ authorizationCode: "code", handoff })).resolves.toBe("completed");
    const record = lifecycle.get(handoff.handoffId);
    expect(record?.state).toBe("active");
    expect(record?.accessToken).toBe("at");
  });

  it("reports retryable_failure without touching the lifecycle for a retryable outcome", async () => {
    const lifecycle = new TokenLifecycleStore();
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan: new Map([[digest("code"), { kind: "retryable_failure", reason: "timeout" }]]) });
    const processor = new OAuthCallbackHandoffProviderProcessor({ provider, lifecycle });
    await expect(processor.process({ authorizationCode: "code", handoff })).resolves.toBe("retryable_failure");
    expect(lifecycle.get(handoff.handoffId)?.state).toBe("pending_exchange");
  });

  it("records a revocation and reports completed on permanent_failure", async () => {
    const lifecycle = new TokenLifecycleStore();
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan: new Map([[digest("code"), { kind: "permanent_failure", reason: "invalid_grant" }]]) });
    const processor = new OAuthCallbackHandoffProviderProcessor({ provider, lifecycle });
    await expect(processor.process({ authorizationCode: "code", handoff })).resolves.toBe("completed");
    const record = lifecycle.get(handoff.handoffId);
    expect(record?.state).toBe("revoked");
    expect(record?.revocationReason).toBe("invalid_grant");
  });

  it("propagates ambiguous-provider errors so the executor keeps the lease", async () => {
    const lifecycle = new TokenLifecycleStore();
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan: new Map([[digest("code"), { kind: "throw", reason: "connection reset" }]]) });
    const processor = new OAuthCallbackHandoffProviderProcessor({ provider, lifecycle });
    await expect(processor.process({ authorizationCode: "code", handoff })).rejects.toThrow("connection reset");
    expect(lifecycle.get(handoff.handoffId)?.state).toBe("pending_exchange");
  });

  it("does not call the provider a second time once the lifecycle has a terminal/active record", async () => {
    const lifecycle = new TokenLifecycleStore();
    const provider = new DeterministicFakeOAuthCallbackProvider({ plan: new Map([[digest("code"), { kind: "exchanged", tokens }]]) });
    const processor = new OAuthCallbackHandoffProviderProcessor({ provider, lifecycle });
    await processor.process({ authorizationCode: "code", handoff });
    await expect(processor.process({ authorizationCode: "code", handoff })).resolves.toBe("completed");
    expect(provider.exchangeCount("code")).toBe(1);
  });
});
