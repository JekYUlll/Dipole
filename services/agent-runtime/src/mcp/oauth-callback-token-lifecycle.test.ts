import { describe, expect, it } from "vitest";
import {
  TokenLifecycleInvalidTransitionError,
  TokenLifecycleStore,
  isTerminalOrActive
} from "./oauth-callback-token-lifecycle.js";

const handoffId = "a".repeat(22);
const bundle = { accessToken: "at-1", refreshToken: "rt-1", tokenType: "Bearer", expiresAt: new Date(Date.now() + 60_000), scope: "read" };

describe("TokenLifecycleStore", () => {
  it("walks pending_exchange → active → refreshed → revoked", () => {
    const store = new TokenLifecycleStore();
    expect(store.markPending(handoffId).state).toBe("pending_exchange");
    expect(store.upsertExchange(handoffId, bundle).state).toBe("active");
    const refreshed = store.refresh(handoffId, { ...bundle, accessToken: "at-2", expiresAt: new Date(Date.now() + 120_000) });
    expect(refreshed.state).toBe("refreshed");
    expect(refreshed.refreshCount).toBe(1);
    const revoked = store.revoke(handoffId, "user_revoked");
    expect(revoked.state).toBe("revoked");
    expect(revoked.accessToken).toBeNull();
    expect(revoked.refreshToken).toBeNull();
    expect(revoked.revocationReason).toBe("user_revoked");
  });

  it("allows expire only from active or refreshed after the expiry has passed", () => {
    const store = new TokenLifecycleStore();
    store.markPending(handoffId);
    const expiredAt = new Date(Date.now() - 1_000);
    store.upsertExchange(handoffId, { ...bundle, expiresAt: expiredAt });
    const expired = store.expire(handoffId, new Date());
    expect(expired.state).toBe("expired");
    expect(expired.accessToken).toBeNull();
  });

  it("rejects illegal transitions", () => {
    const store = new TokenLifecycleStore();
    expect(() => store.refresh(handoffId, bundle)).toThrow(TokenLifecycleInvalidTransitionError);
    store.markPending(handoffId);
    expect(() => store.refresh(handoffId, bundle)).toThrow(TokenLifecycleInvalidTransitionError);
    store.upsertExchange(handoffId, bundle);
    expect(() => store.upsertExchange(handoffId, bundle)).toThrow(TokenLifecycleInvalidTransitionError);
    store.revoke(handoffId, "reason");
    expect(() => store.refresh(handoffId, bundle)).toThrow(TokenLifecycleInvalidTransitionError);
    expect(() => store.revoke(handoffId, "again")).toThrow(TokenLifecycleInvalidTransitionError);
    expect(() => store.expire(handoffId, new Date())).toThrow(TokenLifecycleInvalidTransitionError);
  });

  it("rejects expiring a token that has not yet expired", () => {
    const store = new TokenLifecycleStore();
    store.markPending(handoffId);
    store.upsertExchange(handoffId, { ...bundle, expiresAt: new Date(Date.now() + 60_000) });
    expect(() => store.expire(handoffId, new Date())).toThrow(TokenLifecycleInvalidTransitionError);
  });

  it("rejects invalid handoff ids and revocation reasons", () => {
    const store = new TokenLifecycleStore();
    expect(() => store.markPending("short")).toThrow(TokenLifecycleInvalidTransitionError);
    store.markPending(handoffId);
    store.upsertExchange(handoffId, bundle);
    expect(() => store.revoke(handoffId, "")).toThrow(TokenLifecycleInvalidTransitionError);
  });

  it("returns undefined for unknown handoff and reports isTerminalOrActive correctly", () => {
    const store = new TokenLifecycleStore();
    expect(store.get(handoffId)).toBeUndefined();
    store.markPending(handoffId);
    expect(isTerminalOrActive(store.get(handoffId)!)).toBe(false);
    store.upsertExchange(handoffId, bundle);
    expect(isTerminalOrActive(store.get(handoffId)!)).toBe(true);
    store.revoke(handoffId, "test");
    expect(isTerminalOrActive(store.get(handoffId)!)).toBe(true);
  });
});
