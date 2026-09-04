import { generateKeyPairSync } from "node:crypto";
import { describe, expect, it } from "vitest";

import { openOAuthTokenLifecycleBundle, sealOAuthTokenLifecycleBundle } from "./oauth-token-lifecycle-envelope.js";

describe("OAuth token lifecycle envelope", () => {
  it("binds a canonical token bundle to its handoff and Runtime key", () => {
    const { privateKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
    const expiresAt = new Date("2026-09-04T12:00:00.123Z");
    const sealed = sealOAuthTokenLifecycleBundle({ accessToken: "access", refreshToken: "refresh", tokenType: "Bearer", expiresAt, scope: "calendar.read" }, {
      handoffId: "a".repeat(22), runtimeKeyId: "runtime-key-1", state: "active"
    }, privateKey.export({ format: "pem", type: "pkcs8" }).toString());
    const opened = openOAuthTokenLifecycleBundle(sealed.envelope, {
      handoffId: "a".repeat(22), runtimeKeyId: "runtime-key-1", state: "active", tokenBundleSHA256: sealed.sha256,
      accessTokenExpiresAt: expiresAt.toISOString(), scope: "calendar.read"
    }, privateKey.export({ format: "pem", type: "pkcs8" }).toString());
    expect(opened).toEqual({ accessToken: "access", refreshToken: "refresh", tokenType: "Bearer", expiresAt, scope: "calendar.read" });
  });

  it("rejects a binding with a different handoff", () => {
    const { privateKey } = generateKeyPairSync("rsa", { modulusLength: 2048 });
    const expiresAt = new Date("2026-09-04T12:00:00.123Z");
    const pem = privateKey.export({ format: "pem", type: "pkcs8" }).toString();
    const sealed = sealOAuthTokenLifecycleBundle({ accessToken: "access", tokenType: "Bearer", expiresAt }, { handoffId: "a".repeat(22), runtimeKeyId: "runtime-key-1", state: "active" }, pem);
    expect(() => openOAuthTokenLifecycleBundle(sealed.envelope, {
      handoffId: "b".repeat(22), runtimeKeyId: "runtime-key-1", state: "active", tokenBundleSHA256: sealed.sha256,
      accessTokenExpiresAt: expiresAt.toISOString(), scope: ""
    }, pem)).toThrow("OAuth token lifecycle envelope is invalid");
  });
});
