import { describe, expect, it } from "vitest";

import { createOAuthCallbackHandoffRuntime } from "./oauth-callback-handoff-runtime.js";

const config = Object.freeze({ enabled: true as const, controlSecret: "c".repeat(16), leaseOwner: "runtime-worker-1", keyPaths: Object.freeze({ "runtime-key-1": "/run/secrets/key.pem" }) });

describe("OAuth callback handoff Runtime composition", () => {
  it("uses a caller-supplied processor and keeps the control API handoff-ID-only", async () => {
    const calls: string[] = [];
    const runtime = createOAuthCallbackHandoffRuntime(config, {
      coreSharedSecret: "s".repeat(16),
      rpc: fakeRPC(calls),
      keySource: { async use<T>(_id, operation) { return operation(Buffer.from("key")); } },
      openEnvelope: () => "code",
      processor: { async process({ authorizationCode }) { calls.push(`process:${authorizationCode}`); return "completed"; } }
    });

    await runtime.service.notifyHandoff({ handoffId: "a".repeat(22) });
    expect(runtime.controlSecret).toBe(config.controlSecret);
    expect(calls).toEqual(["claim", "process:code", "complete"]);
  });

  it("rejects an unusable Core credential before creating a control service", () => {
    expect(() => createOAuthCallbackHandoffRuntime(config, {
      coreSharedSecret: "short", rpc: fakeRPC([]), processor: { async process() { return "completed"; } }
    })).toThrow(/Core credential/);
  });
});

function fakeRPC(calls: string[]) {
  return {
    claimOAuthCallbackHandoff(request: any, _metadata: any, _options: any, callback: any) {
      calls.push("claim");
      callback(null, { handoffId: request.handoffId, transactionId: "b".repeat(22), ownerUserId: "c".repeat(22), issuer: "https://auth.example.com", redirectUri: "https://dipole.example.com/oauth/callback", authorizationCodeSha256: "d".repeat(64), sealedAuthorizationCode: "v1.nonce.ciphertext.tag.wrapped-dek", runtimeKeyId: "runtime-key-1", expiresAtUnixMs: BigInt(Date.now() + 300_000), leaseExpiresAtUnixMs: BigInt(Date.now() + 30_000) });
      return {};
    },
    completeOAuthCallbackHandoff(request: any, _metadata: any, _options: any, callback: any) { calls.push("complete"); callback(null, { handoffId: request.handoffId }); return {}; },
    releaseOAuthCallbackHandoff(request: any, _metadata: any, _options: any, callback: any) { calls.push("release"); callback(null, { handoffId: request.handoffId }); return {}; }
  };
}
