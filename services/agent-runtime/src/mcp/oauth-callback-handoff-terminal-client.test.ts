import * as grpc from "@grpc/grpc-js";
import { describe, expect, it } from "vitest";

import type {
  CompleteOAuthCallbackHandoffRequest,
  CompleteOAuthCallbackHandoffResponse,
  ReleaseOAuthCallbackHandoffRequest,
  ReleaseOAuthCallbackHandoffResponse
} from "../generated/dipole/agent/v1/agent.js";
import {
  OAuthCallbackHandoffTerminalClient,
  OAuthCallbackHandoffTerminalDeniedError,
  OAuthCallbackHandoffTerminalInvalidError,
  OAuthCallbackHandoffTerminalUnavailableError,
  type OAuthCallbackHandoffTerminalRPC
} from "./oauth-callback-handoff-terminal-client.js";

const handoffId = "a".repeat(22);

describe("OAuthCallbackHandoffTerminalClient", () => {
  it("completes and releases with a Runtime-only identity", async () => {
    const captured: { complete?: CompleteOAuthCallbackHandoffRequest; release?: ReleaseOAuthCallbackHandoffRequest; metadata?: grpc.Metadata } = {};
    const rpc: OAuthCallbackHandoffTerminalRPC = {
      completeOAuthCallbackHandoff(request, metadata, _options, callback) {
        captured.complete = request;
        captured.metadata = metadata;
        callback(null, { handoffId } satisfies CompleteOAuthCallbackHandoffResponse);
        return {} as grpc.ClientUnaryCall;
      },
      releaseOAuthCallbackHandoff(request, _metadata, _options, callback) {
        captured.release = request;
        callback(null, { handoffId } satisfies ReleaseOAuthCallbackHandoffResponse);
        return {} as grpc.ClientUnaryCall;
      }
    };
    const client = new OAuthCallbackHandoffTerminalClient(rpc, "runtime-secret", 2_000);

    await expect(client.complete({ handoffId, leaseOwner: "runtime-worker-1", requestId: "REQ-1", traceId: "TRACE-1" })).resolves.toBeUndefined();
    await expect(client.release({ handoffId, leaseOwner: "runtime-worker-1" })).resolves.toBeUndefined();
    expect(captured.complete).toMatchObject({ handoffId, leaseOwner: "runtime-worker-1", context: { callerService: "dipole-agent", principalUserId: "", requestId: "REQ-1", traceId: "TRACE-1" } });
    expect(captured.release).toMatchObject({ handoffId, leaseOwner: "runtime-worker-1", context: { callerService: "dipole-agent", principalUserId: "" } });
    expect(captured.metadata?.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
    expect(captured.metadata?.get("x-dipole-service-token")).toEqual(["runtime-secret"]);
  });

  it("fails closed for invalid input, conflicting evidence, and Core errors", async () => {
    const cases: readonly { readonly response?: CompleteOAuthCallbackHandoffResponse; readonly error?: grpc.ServiceError; readonly expected: Error }[] = [
      { response: { handoffId: "b".repeat(22) }, expected: new OAuthCallbackHandoffTerminalUnavailableError() },
      { error: grpcError(grpc.status.PERMISSION_DENIED), expected: new OAuthCallbackHandoffTerminalDeniedError() },
      { error: grpcError(grpc.status.INVALID_ARGUMENT), expected: new OAuthCallbackHandoffTerminalInvalidError() }
    ];
    for (const testCase of cases) {
      const rpc: OAuthCallbackHandoffTerminalRPC = {
        completeOAuthCallbackHandoff(_request, _metadata, _options, callback) {
          callback(testCase.error ?? null, testCase.response);
          return {} as grpc.ClientUnaryCall;
        },
        releaseOAuthCallbackHandoff(_request, _metadata, _options, callback) {
          callback(testCase.error ?? null, testCase.response);
          return {} as grpc.ClientUnaryCall;
        }
      };
      const client = new OAuthCallbackHandoffTerminalClient(rpc, "runtime-secret");
      await expect(client.complete({ handoffId, leaseOwner: "runtime-worker-1" })).rejects.toBeInstanceOf(testCase.expected.constructor);
      await expect(client.release({ handoffId, leaseOwner: "runtime-worker-1" })).rejects.toBeInstanceOf(testCase.expected.constructor);
    }
    const client = new OAuthCallbackHandoffTerminalClient({} as OAuthCallbackHandoffTerminalRPC, "runtime-secret");
    await expect(client.complete({ handoffId: "short", leaseOwner: "runtime-worker-1" })).rejects.toBeInstanceOf(OAuthCallbackHandoffTerminalInvalidError);
  });
});

function grpcError(code: grpc.status): grpc.ServiceError {
  return Object.assign(new Error("RPC failed"), { code, details: "RPC failed", metadata: new grpc.Metadata() }) as grpc.ServiceError;
}
