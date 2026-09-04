import { describe, expect, it } from "vitest";
import { isValidTokenBundle, type OAuthTokenBundle } from "./oauth-callback-provider.js";

describe("isValidTokenBundle", () => {
  const base: OAuthTokenBundle = { accessToken: "at", tokenType: "Bearer", expiresAt: new Date(Date.now() + 60_000) };

  it("accepts a minimal valid bundle", () => {
    expect(isValidTokenBundle(base)).toBe(true);
  });

  it("accepts an optional refresh token and scope", () => {
    expect(isValidTokenBundle({ ...base, refreshToken: "rt", scope: "read write" })).toBe(true);
  });

  it("rejects empty or overlong string fields", () => {
    expect(isValidTokenBundle({ ...base, accessToken: "" })).toBe(false);
    expect(isValidTokenBundle({ ...base, tokenType: "" })).toBe(false);
    expect(isValidTokenBundle({ ...base, accessToken: "x".repeat(8193) })).toBe(false);
    expect(isValidTokenBundle({ ...base, refreshToken: "" })).toBe(false);
  });

  it("rejects invalid expiry dates", () => {
    expect(isValidTokenBundle({ ...base, expiresAt: new Date(Number.NaN) })).toBe(false);
  });

  it("rejects an oversized scope string", () => {
    expect(isValidTokenBundle({ ...base, scope: "x".repeat(2049) })).toBe(false);
  });
});
