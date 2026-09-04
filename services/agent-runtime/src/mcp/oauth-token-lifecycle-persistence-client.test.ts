import * as grpc from "@grpc/grpc-js";
import { generateKeyPairSync } from "node:crypto";
import { describe, expect, it } from "vitest";

import type { OAuthCallbackHandoffClaim } from "./oauth-callback-handoff-claim-client.js";
import { OAuthTokenLifecyclePersistenceClient, type OAuthTokenLifecyclePersistenceRPC } from "./oauth-token-lifecycle-persistence-client.js";

const handoff = Object.freeze({
  handoffId: "a".repeat(22), transactionId: "b".repeat(22), ownerUserId: "c".repeat(22), issuer: "https://auth.example.com", redirectUri: "https://dipole.example.com/callback",
  authorizationCodeSHA256: "d".repeat(64), sealedAuthorizationCode: "v1.nonce.ciphertext.tag.wrapped", runtimeKeyId: "runtime-key-1",
  expiresAt: new Date(Date.now() + 300_000), leaseExpiresAt: new Date(Date.now() + 30_000)
}) satisfies OAuthCallbackHandoffClaim;

describe("OAuthTokenLifecyclePersistenceClient", () => {
  it("seals a bundle before submitting only opaque lifecycle evidence", async () => {
    const { privateKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
    const pem = Buffer.from(privateKey.export({ format: "pem", type: "pkcs8" }).toString());
    const captured: { request?: any; metadata?: grpc.Metadata } = {};
    const rpc: OAuthTokenLifecyclePersistenceRPC = {
      persistOAuthTokenLifecycle(request, metadata, _options, callback) {
        captured.request = request; captured.metadata = metadata;
        callback(null, { handoffId: request.handoffId, state: request.state });
        return {} as grpc.ClientUnaryCall;
      }
    };
    const client = new OAuthTokenLifecyclePersistenceClient(rpc, "runtime-secret", { async use(_id, operation) { return operation(Buffer.from(pem)); } });
    await client.persistActive({ handoff, leaseOwner: "runtime-worker-1", requestId: "REQ-1", bundle: { accessToken: "access-token", refreshToken: "refresh-token", tokenType: "Bearer", expiresAt: new Date(Date.now() + 60_000), scope: "calendar.read" } });
    expect(captured.request).toMatchObject({ handoffId: handoff.handoffId, leaseOwner: "runtime-worker-1", state: "active", tokenBundleSha256: expect.stringMatching(/^[a-f0-9]{64}$/u), scope: "calendar.read" });
    expect(captured.request.sealedTokenBundle).toMatch(/^v1\./u);
    expect(captured.request.sealedTokenBundle).not.toContain("access-token");
    expect(captured.metadata?.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
  });
});
