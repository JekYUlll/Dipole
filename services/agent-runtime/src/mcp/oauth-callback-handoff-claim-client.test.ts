import * as grpc from "@grpc/grpc-js";
import { describe, expect, it } from "vitest";

import type { ClaimOAuthCallbackHandoffRequest, ClaimOAuthCallbackHandoffResponse } from "../generated/dipole/agent/v1/agent.js";
import {
  OAuthCallbackHandoffClaimClient,
  OAuthCallbackHandoffClaimDeniedError,
  OAuthCallbackHandoffClaimInvalidError,
  OAuthCallbackHandoffClaimUnavailableError,
  type OAuthCallbackHandoffClaimRPC
} from "./oauth-callback-handoff-claim-client.js";

const handoffId = "a".repeat(22);
const transactionId = "b".repeat(22);
const response: ClaimOAuthCallbackHandoffResponse = {
  handoffId,
  transactionId,
  issuer: "https://auth.example.com/tenant",
  redirectUri: "https://dipole.example.com/oauth/callback",
  authorizationCodeSha256: "c".repeat(64),
  sealedAuthorizationCode: "v1.nonce.ciphertext.tag.wrapped-dek",
  runtimeKeyId: "runtime-key-1",
  expiresAtUnixMs: BigInt(Date.now() + 300_000),
  leaseExpiresAtUnixMs: BigInt(Date.now() + 30_000)
};

describe("OAuthCallbackHandoffClaimClient", () => {
  it("binds the Agent Runtime caller and returns only validated opaque material", async () => {
    let captured: { request?: ClaimOAuthCallbackHandoffRequest; metadata?: grpc.Metadata } = {};
    const rpc: OAuthCallbackHandoffClaimRPC = {
      claimOAuthCallbackHandoff(request, metadata, _options, callback) {
        captured = { request, metadata: metadata as grpc.Metadata };
        callback(null, response);
        return {} as grpc.ClientUnaryCall;
      }
    };
    const client = new OAuthCallbackHandoffClaimClient(rpc, "runtime-secret", 2_000);

    await expect(client.claim({ handoffId, leaseOwner: "runtime-worker-1", requestId: "REQ-1", traceId: "TRACE-1" })).resolves.toMatchObject({
      handoffId, transactionId, sealedAuthorizationCode: response.sealedAuthorizationCode
    });
    expect(captured.request).toMatchObject({ handoffId, leaseOwner: "runtime-worker-1", context: { callerService: "dipole-agent", principalUserId: "", requestId: "REQ-1", traceId: "TRACE-1" } });
    expect(captured.metadata?.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
    expect(captured.metadata?.get("x-dipole-service-token")).toEqual(["runtime-secret"]);
  });

  it("fails closed for invalid input, conflicting records, and service errors", async () => {
    const cases: readonly { readonly name: string; readonly response?: ClaimOAuthCallbackHandoffResponse; readonly error?: grpc.ServiceError; readonly expected: Error }[] = [
      { name: "conflicting handoff", response: { ...response, handoffId: transactionId }, expected: new OAuthCallbackHandoffClaimUnavailableError() },
      { name: "expired lease", response: { ...response, leaseExpiresAtUnixMs: BigInt(1) }, expected: new OAuthCallbackHandoffClaimUnavailableError() },
      { name: "denied", error: grpcError(grpc.status.PERMISSION_DENIED), expected: new OAuthCallbackHandoffClaimDeniedError() },
      { name: "invalid", error: grpcError(grpc.status.INVALID_ARGUMENT), expected: new OAuthCallbackHandoffClaimInvalidError() }
    ];
    for (const testCase of cases) {
      const rpc: OAuthCallbackHandoffClaimRPC = {
        claimOAuthCallbackHandoff(_request, _metadata, _options, callback) {
          callback(testCase.error ?? null, testCase.response);
          return {} as grpc.ClientUnaryCall;
        }
      };
      const client = new OAuthCallbackHandoffClaimClient(rpc, "runtime-secret");
      await expect(client.claim({ handoffId, leaseOwner: "runtime-worker-1" })).rejects.toBeInstanceOf(testCase.expected.constructor);
    }
    const client = new OAuthCallbackHandoffClaimClient({} as OAuthCallbackHandoffClaimRPC, "runtime-secret");
    await expect(client.claim({ handoffId: "short", leaseOwner: "runtime-worker-1" })).rejects.toBeInstanceOf(OAuthCallbackHandoffClaimInvalidError);
  });
});

function grpcError(code: grpc.status): grpc.ServiceError {
  return Object.assign(new Error("RPC failed"), { code, details: "RPC failed", metadata: new grpc.Metadata() }) as grpc.ServiceError;
}
