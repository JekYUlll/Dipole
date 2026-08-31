const leaseOwnerPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/;

export type OAuthCallbackRuntimeConfig =
  | Readonly<{ enabled: false }>
  | Readonly<{ enabled: true; controlSecret: string; leaseOwner: string; keyPaths: Readonly<Record<string, string>> }>;

/** Parses the explicit, default-disabled Runtime callback assembly contract. */
export function loadOAuthCallbackRuntimeConfig(env: NodeJS.ProcessEnv): OAuthCallbackRuntimeConfig {
  if (env.DIPOLE_AGENT_OAUTH_CALLBACK_ENABLED?.trim().toLowerCase() !== "true") return Object.freeze({ enabled: false });
  const controlSecret = env.DIPOLE_AGENT_OAUTH_CALLBACK_CONTROL_SECRET?.trim() ?? "";
  const leaseOwner = env.DIPOLE_AGENT_OAUTH_CALLBACK_LEASE_OWNER?.trim() ?? "";
  if (controlSecret.length < 16 || !leaseOwnerPattern.test(leaseOwner)) throw new Error("OAuth callback Runtime configuration is invalid");
  let keyPaths: unknown;
  try { keyPaths = JSON.parse(env.DIPOLE_AGENT_OAUTH_CALLBACK_RUNTIME_KEYS_JSON ?? ""); } catch { throw new Error("OAuth callback Runtime configuration is invalid"); }
  if (keyPaths === null || typeof keyPaths !== "object" || Array.isArray(keyPaths) || Object.keys(keyPaths).length < 1) throw new Error("OAuth callback Runtime configuration is invalid");
  return Object.freeze({ enabled: true, controlSecret, leaseOwner, keyPaths: Object.freeze({ ...(keyPaths as Record<string, string>) }) });
}

/** This Runtime image has no approved provider exchange processor yet. */
export function assertOAuthCallbackRuntimeUnavailable(config: OAuthCallbackRuntimeConfig): void {
  if (config.enabled) throw new Error("OAuth callback Runtime requires an approved provider processor deployment profile");
}
