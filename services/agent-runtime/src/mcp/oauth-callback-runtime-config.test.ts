import { describe, expect, it } from "vitest";
import { assertOAuthCallbackRuntimeUnavailable, loadOAuthCallbackRuntimeConfig } from "./oauth-callback-runtime-config.js";

describe("OAuth callback Runtime config", () => {
  it("is disabled unless explicitly enabled", () => {
    expect(loadOAuthCallbackRuntimeConfig({ DIPOLE_AGENT_OAUTH_CALLBACK_CONTROL_SECRET: "x".repeat(16) })).toEqual({ enabled: false });
  });
  it("requires a secret, safe lease owner, and non-empty key mapping", () => {
    const env = { DIPOLE_AGENT_OAUTH_CALLBACK_ENABLED: "true", DIPOLE_AGENT_OAUTH_CALLBACK_CONTROL_SECRET: "s".repeat(16), DIPOLE_AGENT_OAUTH_CALLBACK_LEASE_OWNER: "runtime-1", DIPOLE_AGENT_OAUTH_CALLBACK_RUNTIME_KEYS_JSON: '{"runtime-key-1":"/run/secrets/key.pem"}' };
    expect(loadOAuthCallbackRuntimeConfig(env)).toMatchObject({ enabled: true, leaseOwner: "runtime-1" });
    expect(() => loadOAuthCallbackRuntimeConfig({ ...env, DIPOLE_AGENT_OAUTH_CALLBACK_RUNTIME_KEYS_JSON: "{}" })).toThrow(/configuration/);
  });
  it("rejects explicit enablement until an approved processor profile exists", () => {
    const config = loadOAuthCallbackRuntimeConfig({ DIPOLE_AGENT_OAUTH_CALLBACK_ENABLED: "true", DIPOLE_AGENT_OAUTH_CALLBACK_CONTROL_SECRET: "s".repeat(16), DIPOLE_AGENT_OAUTH_CALLBACK_LEASE_OWNER: "runtime-1", DIPOLE_AGENT_OAUTH_CALLBACK_RUNTIME_KEYS_JSON: '{"runtime-key-1":"/run/secrets/key.pem"}' });
    expect(() => assertOAuthCallbackRuntimeUnavailable(config)).toThrow(/approved provider processor/);
    expect(() => assertOAuthCallbackRuntimeUnavailable({ enabled: false })).not.toThrow();
  });
});
