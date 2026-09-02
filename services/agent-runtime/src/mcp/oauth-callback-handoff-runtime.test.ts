import { describe, expect, it } from "vitest";
import * as grpc from "@grpc/grpc-js";

import { createOAuthCallbackHandoffRuntime } from "./oauth-callback-handoff-runtime.js";

const config = Object.freeze({ enabled: true as const, controlSecret: "c".repeat(16), leaseOwner: "runtime-worker-1", keyPaths: Object.freeze({ "runtime-key-1": "/run/secrets/key.pem" }) });

describe("OAuth callback handoff Runtime composition", () => {
  it("uses a caller-supplied processor and keeps the control API handoff-ID-only", async () => {
    const calls: string[] = [];
    const runtime = createOAuthCallbackHandoffRuntime(config, {
      coreSharedSecret: "s".repeat(16),
      rpc: fakeRPC(calls) as never,
      keySource: { async use<T>(_id: string, operation: (key: Buffer) => Promise<T> | T): Promise<T> { return operation(Buffer.from("key")); } },
      openEnvelope: () => "code",
      processor: { async process({ authorizationCode }) { calls.push(`process:${authorizationCode}`); return "completed"; } }
    });

    await runtime.service.notifyHandoff({ handoffId: "a".repeat(22) });
    expect(runtime.controlSecret).toBe(config.controlSecret);
    expect(calls).toEqual(["claim", "process:code", "complete"]);
  });

  it("rejects an unusable Core credential before creating a control service", () => {
    expect(() => createOAuthCallbackHandoffRuntime(config, {
      coreSharedSecret: "short", rpc: fakeRPC([]) as never, processor: { async process() { return "completed"; } }
    })).toThrow(/Core credential/);
  });

  it("allows a replacement Runtime to claim only after the previous Runtime releases its Core lease", async () => {
    const calls: string[] = [];
    const rpc = leaseAwareRPC(calls);
    const first = createOAuthCallbackHandoffRuntime({ ...config, leaseOwner: "runtime-a" }, runtimeDependencies(rpc, {
      async process({ authorizationCode }) { calls.push(`process-a:${authorizationCode}`); return "retryable_failure"; }
    }));
    const replacement = createOAuthCallbackHandoffRuntime({ ...config, leaseOwner: "runtime-b" }, runtimeDependencies(rpc, {
      async process({ authorizationCode }) { calls.push(`process-b:${authorizationCode}`); return "completed"; }
    }));

    await first.service.notifyHandoff({ handoffId: "a".repeat(22) });
    await replacement.service.notifyHandoff({ handoffId: "a".repeat(22) });

    expect(calls).toEqual([
      "claim:runtime-a", "process-a:code", "release:runtime-a",
      "claim:runtime-b", "process-b:code", "complete:runtime-b"
    ]);
  });
});

function runtimeDependencies(rpc: unknown, processor: { process(input: { authorizationCode: string }): Promise<"completed" | "retryable_failure"> }) {
  return {
    coreSharedSecret: "s".repeat(16),
    rpc: rpc as never,
    keySource: { async use<T>(_id: string, operation: (key: Buffer) => Promise<T> | T): Promise<T> { return operation(Buffer.from("key")); } },
    openEnvelope: () => "code",
    processor
  };
}

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

function leaseAwareRPC(calls: string[]) {
  let leaseOwner: string | undefined;
  let completed = false;
  const handoffId = "a".repeat(22);
  const claimResponse = (owner: string) => ({
    handoffId, transactionId: "b".repeat(22), ownerUserId: "c".repeat(22), issuer: "https://auth.example.com",
    redirectUri: "https://dipole.example.com/oauth/callback", authorizationCodeSha256: "d".repeat(64),
    sealedAuthorizationCode: "v1.nonce.ciphertext.tag.wrapped-dek", runtimeKeyId: "runtime-key-1",
    expiresAtUnixMs: BigInt(Date.now() + 300_000), leaseExpiresAtUnixMs: BigInt(Date.now() + 30_000)
  });
  const leaseDenied = (callback: (error: Error | null) => void) => callback(Object.assign(new Error("lease unavailable"), { code: grpc.status.FAILED_PRECONDITION }) as never);

  return {
    claimOAuthCallbackHandoff(request: any, _metadata: any, _options: any, callback: any) {
      if (completed || leaseOwner !== undefined) {
        leaseDenied(callback);
        return {};
      }
      const owner = request.leaseOwner as string;
      leaseOwner = owner;
      calls.push(`claim:${owner}`);
      callback(null, claimResponse(owner));
      return {};
    },
    completeOAuthCallbackHandoff(request: any, _metadata: any, _options: any, callback: any) {
      if (request.leaseOwner !== leaseOwner) {
        leaseDenied(callback);
        return {};
      }
      completed = true;
      calls.push(`complete:${request.leaseOwner}`);
      callback(null, { handoffId: request.handoffId });
      return {};
    },
    releaseOAuthCallbackHandoff(request: any, _metadata: any, _options: any, callback: any) {
      if (request.leaseOwner !== leaseOwner) {
        leaseDenied(callback);
        return {};
      }
      calls.push(`release:${request.leaseOwner}`);
      leaseOwner = undefined;
      callback(null, { handoffId: request.handoffId });
      return {};
    }
  };
}
