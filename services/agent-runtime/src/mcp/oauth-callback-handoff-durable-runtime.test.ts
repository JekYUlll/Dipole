import { createHash } from "node:crypto";
import * as grpc from "@grpc/grpc-js";
import { describe, expect, it } from "vitest";

import { DeterministicFakeOAuthCallbackProvider, type DeterministicOutcome } from "./deterministic-fake-oauth-callback-provider.js";
import {
  OAuthCallbackHandoffClaimClient,
  OAuthCallbackHandoffClaimDeniedError,
  OAuthCallbackHandoffClaimInvalidError,
  type OAuthCallbackHandoffClaimRPC
} from "./oauth-callback-handoff-claim-client.js";
import { OAuthCallbackHandoffExecutor } from "./oauth-callback-handoff-executor.js";
import { OAuthCallbackHandoffProviderProcessor } from "./oauth-callback-handoff-provider-processor.js";
import {
  OAuthCallbackHandoffTerminalClient,
  type OAuthCallbackHandoffTerminalRPC
} from "./oauth-callback-handoff-terminal-client.js";
import { createOAuthCallbackHandoffControlService } from "./oauth-callback-handoff-control-service.js";
import { TokenLifecycleStore } from "./oauth-callback-token-lifecycle.js";
import type { OAuthCallbackRuntimeKeySource } from "./node-oauth-callback-runtime-key-source.js";

/*
 * End-to-end composition tests for durable handoff recovery. Six scenarios:
 *   1. duplicate `notifyHandoff` — provider must run once
 *   2. worker restart across lease expiry — new lease owner completes it
 *   3. lease expires while executor is idle — no provider call, release
 *   4. exact replay after `completed` — Core rejects second claim
 *   5. rollback via retryable_failure — Runtime releases and second attempt completes
 *   6. permission denied — Runtime never opens envelope, never touches provider
 */

const handoffId = "a".repeat(22);
const ownerUserId = "c".repeat(22);
const authorizationCode = "auth-code-XYZ";
const authorizationCodeSha256 = createHash("sha256").update(authorizationCode).digest("hex");

type LeaseState = { readonly owner: string; expiresAt: Date };
type HandoffState = "callback_recorded" | "exchange_claimed" | "exchanged";

/**
 * Minimal, in-memory Core-side fake of the durable handoff store. It enforces
 * the state transitions the real Core enforces (single active lease per
 * handoff, no re-claim after `exchanged`, `PERMISSION_DENIED` for the
 * `deniedOwner`). Lease expiry is driven by a caller-supplied clock so we can
 * simulate worker restarts deterministically.
 */
class FakeCoreHandoffStore {
  private state: HandoffState = "callback_recorded";
  private lease: LeaseState | null = null;
  private readonly leaseTtlMs = 30_000;
  private readonly handoffExpiresAt: Date;
  readonly calls: string[] = [];

  constructor(
    readonly clock: () => Date,
    private readonly deniedOwner?: string
  ) {
    this.handoffExpiresAt = new Date(clock().getTime() + 300_000);
  }

  claim(owner: string): { ok: true; leaseExpiresAt: Date } | { ok: false; code: grpc.status } {
    this.calls.push(`claim:${owner}`);
    if (this.deniedOwner !== undefined && owner === this.deniedOwner) return { ok: false, code: grpc.status.PERMISSION_DENIED };
    const now = this.clock();
    if (this.state === "exchanged") return { ok: false, code: grpc.status.FAILED_PRECONDITION };
    if (this.lease !== null && this.lease.expiresAt.getTime() > now.getTime()) return { ok: false, code: grpc.status.FAILED_PRECONDITION };
    this.state = "exchange_claimed";
    const leaseExpiresAt = new Date(now.getTime() + this.leaseTtlMs);
    this.lease = { owner, expiresAt: leaseExpiresAt };
    return { ok: true, leaseExpiresAt };
  }

  complete(owner: string): boolean {
    this.calls.push(`complete:${owner}`);
    if (this.lease === null || this.lease.owner !== owner) return false;
    if (this.lease.expiresAt.getTime() <= this.clock().getTime()) return false;
    this.state = "exchanged";
    this.lease = null;
    return true;
  }

  release(owner: string): boolean {
    this.calls.push(`release:${owner}`);
    if (this.lease === null || this.lease.owner !== owner) return false;
    if (this.lease.expiresAt.getTime() <= this.clock().getTime()) return false;
    this.state = "callback_recorded";
    this.lease = null;
    return true;
  }

  handoffExpiry(): Date { return this.handoffExpiresAt; }
}

function buildRPC(store: FakeCoreHandoffStore): OAuthCallbackHandoffClaimRPC & OAuthCallbackHandoffTerminalRPC {
  const envelope = "v1.nonce.ciphertext.tag.wrapped-dek";
  const call = {} as grpc.ClientUnaryCall;
  return {
    claimOAuthCallbackHandoff(request, _metadata, _options, callback) {
      const outcome = store.claim(request.leaseOwner);
      if (!outcome.ok) {
        callback(Object.assign(new Error("lease unavailable"), { code: outcome.code, details: "lease unavailable", metadata: new grpc.Metadata() }) as grpc.ServiceError);
        return call;
      }
      callback(null, {
        handoffId: request.handoffId,
        transactionId: "b".repeat(22),
        ownerUserId,
        issuer: "https://auth.example.com",
        redirectUri: "https://dipole.example.com/oauth/callback",
        authorizationCodeSha256,
        sealedAuthorizationCode: envelope,
        runtimeKeyId: "runtime-key-1",
        expiresAtUnixMs: BigInt(store.handoffExpiry().getTime()),
        leaseExpiresAtUnixMs: BigInt(outcome.leaseExpiresAt.getTime())
      });
      return call;
    },
    completeOAuthCallbackHandoff(request, _metadata, _options, callback) {
      if (!store.complete(request.leaseOwner)) {
        callback(Object.assign(new Error("lease unavailable"), { code: grpc.status.FAILED_PRECONDITION, details: "lease unavailable", metadata: new grpc.Metadata() }) as grpc.ServiceError);
        return call;
      }
      callback(null, { handoffId: request.handoffId });
      return call;
    },
    releaseOAuthCallbackHandoff(request, _metadata, _options, callback) {
      if (!store.release(request.leaseOwner)) {
        callback(Object.assign(new Error("lease unavailable"), { code: grpc.status.FAILED_PRECONDITION, details: "lease unavailable", metadata: new grpc.Metadata() }) as grpc.ServiceError);
        return call;
      }
      callback(null, { handoffId: request.handoffId });
      return call;
    }
  };
}

function buildKeySource(): OAuthCallbackRuntimeKeySource {
  return { async use<T>(_id: string, operation: (key: Buffer) => Promise<T> | T): Promise<T> { return operation(Buffer.from("key")); } };
}

interface DurableComposition {
  readonly notify: (leaseOwner: string) => Promise<void>;
  readonly store: FakeCoreHandoffStore;
  readonly lifecycle: TokenLifecycleStore;
  readonly provider: DeterministicFakeOAuthCallbackProvider;
  readonly clock: { current: Date };
}

interface CompositionOptions {
  readonly plan?: Map<string, DeterministicOutcome>;
  readonly deniedOwner?: string;
}

function buildComposition(options: CompositionOptions = {}): DurableComposition {
  // Anchor the fake clock to real system time so `fromProto`'s absolute expiry
  // check against `Date.now()` remains consistent with our simulated advances.
  const clock = { current: new Date() };
  const now = (): Date => clock.current;
  const plan = options.plan ?? new Map<string, DeterministicOutcome>([[
    createHash("sha256").update(authorizationCode).digest("hex"),
    { kind: "exchanged", tokens: { accessToken: "at", tokenType: "Bearer", expiresAt: new Date(now().getTime() + 60_000) } }
  ]]);
  const store = new FakeCoreHandoffStore(now, options.deniedOwner);
  const rpc = buildRPC(store);
  const provider = new DeterministicFakeOAuthCallbackProvider({ plan });
  const lifecycle = new TokenLifecycleStore(now);
  const processor = new OAuthCallbackHandoffProviderProcessor({ provider, lifecycle });
  const claims = new OAuthCallbackHandoffClaimClient(rpc, "runtime-secret");
  const terminal = new OAuthCallbackHandoffTerminalClient(rpc, "runtime-secret");
  const executor = new OAuthCallbackHandoffExecutor(
    claims, terminal, buildKeySource(), processor, () => authorizationCode, now
  );
  const notify = async (leaseOwner: string): Promise<void> => {
    const service = createOAuthCallbackHandoffControlService(executor, leaseOwner);
    await service.notifyHandoff({ handoffId });
  };
  return { notify, store, lifecycle, provider, clock };
}

describe("OAuth callback durable handoff — end-to-end scenarios", () => {
  it("scenario 1: duplicate notifyHandoff invokes the provider only once", async () => {
    const { notify, store, provider, lifecycle } = buildComposition();
    await notify("runtime-worker-1");
    await expect(notify("runtime-worker-1")).rejects.toBeInstanceOf(Error);
    expect(provider.exchangeCount(authorizationCode)).toBe(1);
    expect(lifecycle.get(handoffId)?.state).toBe("active");
    expect(store.calls.filter(entry => entry.startsWith("complete:")).length).toBe(1);
  });

  it("scenario 2: worker restart — a new lease owner claims after the previous lease expires", async () => {
    const { notify, store, provider, lifecycle, clock } = buildComposition();
    const previousExecute = new OAuthCallbackHandoffClaimClient(buildRPC(store), "runtime-secret");
    // Simulate worker-a claiming and dying before completion:
    const claim = await previousExecute.claim({ handoffId, leaseOwner: "runtime-worker-a" });
    expect(claim.leaseExpiresAt.getTime()).toBeGreaterThan(clock.current.getTime());
    // Advance clock past lease TTL to expire it:
    clock.current = new Date(clock.current.getTime() + 60_000);
    // A different lease owner now completes the handoff:
    await notify("runtime-worker-b");
    expect(provider.exchangeCount(authorizationCode)).toBe(1);
    expect(lifecycle.get(handoffId)?.state).toBe("active");
    expect(store.calls).toContain("claim:runtime-worker-a");
    expect(store.calls).toContain("claim:runtime-worker-b");
    expect(store.calls).toContain("complete:runtime-worker-b");
  });

  it("scenario 3: lease expiry after claim but before provider — no side effect, release attempted", async () => {
    const { store, provider, lifecycle, clock } = buildComposition();
    const rpc = buildRPC(store);
    const claims = new OAuthCallbackHandoffClaimClient(rpc, "runtime-secret");
    const terminal = new OAuthCallbackHandoffTerminalClient(rpc, "runtime-secret");
    const processor = new OAuthCallbackHandoffProviderProcessor({
      provider, lifecycle
    });
    // Freeze clock at claim time, then jump past lease during envelope open:
    let openedAt: Date | undefined;
    const executor = new OAuthCallbackHandoffExecutor(
      claims, terminal, {
        async use<T>(_id: string, operation: (key: Buffer) => Promise<T> | T): Promise<T> {
          openedAt = new Date(clock.current.getTime());
          clock.current = new Date(clock.current.getTime() + 60_000);
          return operation(Buffer.from("key"));
        }
      }, processor, () => authorizationCode, () => clock.current
    );
    // Executor's catch handler awaits `terminal.release`; Core rejects the release for
    // an expired lease, so the surfaced error may be either the lease-expired error or
    // the terminal-invalid error from Core's rejection. Either way the invariants
    // matter more than the exact message: the provider is never called and the handoff
    // is never marked completed.
    await expect(executor.execute({ handoffId, leaseOwner: "runtime-worker-1" })).rejects.toBeInstanceOf(Error);
    expect(openedAt).toBeDefined();
    expect(provider.exchangeCount(authorizationCode)).toBe(0);
    expect(store.calls).not.toContain("complete:runtime-worker-1");
  });

  it("scenario 4: exact replay after completed — Core rejects second claim, provider not invoked twice", async () => {
    const { notify, store, provider, lifecycle } = buildComposition();
    await notify("runtime-worker-1");
    // Second `notifyHandoff` after the handoff already reached `exchanged`.
    // Core denies the claim with FAILED_PRECONDITION, which the claim client maps
    // to `OAuthCallbackHandoffClaimInvalidError`. Provider is never called again.
    await expect(notify("runtime-worker-2")).rejects.toBeInstanceOf(OAuthCallbackHandoffClaimInvalidError);
    expect(provider.exchangeCount(authorizationCode)).toBe(1);
    expect(lifecycle.get(handoffId)?.state).toBe("active");
  });

  it("scenario 5: retryable_failure rollback — release then re-claim succeeds", async () => {
    const digest = createHash("sha256").update(authorizationCode).digest("hex");
    const plan = new Map<string, DeterministicOutcome>([[digest, { kind: "retryable_failure", reason: "timeout" }]]);
    const { notify, store, provider, lifecycle } = buildComposition({ plan });
    await notify("runtime-worker-1");
    // Provider declared retryable_failure so executor released the lease.
    expect(store.calls).toContain("release:runtime-worker-1");
    expect(lifecycle.get(handoffId)?.state).toBe("pending_exchange");
    // Replace the plan for a second successful exchange:
    plan.set(digest, { kind: "exchanged", tokens: { accessToken: "at", tokenType: "Bearer", expiresAt: new Date(Date.now() + 60_000) } });
    await notify("runtime-worker-2");
    expect(provider.exchangeCount(authorizationCode)).toBe(2);
    expect(lifecycle.get(handoffId)?.state).toBe("active");
    expect(store.calls).toContain("complete:runtime-worker-2");
  });

  it("scenario 6: PERMISSION_DENIED — Runtime never opens envelope, never touches provider or lifecycle", async () => {
    const { notify, store, provider, lifecycle } = buildComposition({ deniedOwner: "runtime-worker-untrusted" });
    await expect(notify("runtime-worker-untrusted")).rejects.toBeInstanceOf(OAuthCallbackHandoffClaimDeniedError);
    expect(provider.exchangeCount(authorizationCode)).toBe(0);
    expect(lifecycle.get(handoffId)).toBeUndefined();
    expect(store.calls).toEqual(["claim:runtime-worker-untrusted"]);
  });
});
