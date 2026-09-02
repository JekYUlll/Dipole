import { describe, expect, it } from "vitest";

import {
  createOAuthAuthorizationTransaction,
  oauthAuthorizationStateSHA256,
  openConsumedOAuthAuthorizationTransaction
} from "./oauth-authorization-transaction.js";

const key = Buffer.alloc(32, 7);
const state = "A".repeat(43);
const verifier = "B".repeat(43);
const base = Object.freeze({
  ownerUserId: "U100", issuer: "https://auth.example.com/tenant-a", redirectUri: "https://dipole.example.com/oauth/callback",
  state, codeVerifier: verifier, encryptionKey: key, now: new Date("2026-08-31T00:00:00.000Z"), randomBytes: (size: number) => Buffer.alloc(size, 9)
});

describe("OAuth authorization transaction contract", () => {
  it("seals the verifier and stores only a state digest with immutable callback binding", () => {
    const transaction = createOAuthAuthorizationTransaction(base);
    expect(transaction).toMatchObject({ ownerUserId: "U100", stateSHA256: oauthAuthorizationStateSHA256(state), expiresAt: "2026-08-31T00:10:00.000Z" });
    expect(JSON.stringify(transaction)).not.toContain(verifier);
    expect(JSON.stringify(transaction)).not.toContain(state);
    expect(transaction.sealedCodeVerifier).toMatch(/^v1\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/);
  });

  it("opens only the matching owner, state, binding, and unexpired record", () => {
    const transaction = createOAuthAuthorizationTransaction(base);
    expect(openConsumedOAuthAuthorizationTransaction(transaction, "U100", state, key, new Date("2026-08-31T00:09:59.999Z"))).toMatchObject({ codeVerifier: verifier });
    expect(() => openConsumedOAuthAuthorizationTransaction(transaction, "U200", state, key, base.now)).toThrow(/owner/i);
    expect(() => openConsumedOAuthAuthorizationTransaction(transaction, "U100", "C".repeat(43), key, base.now)).toThrow(/state/i);
    expect(() => openConsumedOAuthAuthorizationTransaction(transaction, "U100", state, key, new Date("2026-08-31T00:10:00.000Z"))).toThrow(/expired/i);
  });

  it("rejects tampering, malformed records, and invalid keys", () => {
    const transaction = createOAuthAuthorizationTransaction(base);
    expect(() => openConsumedOAuthAuthorizationTransaction({ ...transaction, redirectUri: "https://dipole.example.com/other" }, "U100", state, key, base.now)).toThrow(/verifier/i);
    expect(() => createOAuthAuthorizationTransaction({ ...base, encryptionKey: Buffer.alloc(31) })).toThrow(/key/i);
    expect(() => createOAuthAuthorizationTransaction({ ...base, state: "short" })).toThrow(/state/i);
  });
});
