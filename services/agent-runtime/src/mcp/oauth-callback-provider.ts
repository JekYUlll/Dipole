// OAuth callback provider seam.
//
// Purpose: define the narrow contract the Runtime uses to exchange a Runtime-decrypted
// authorization code for a token bundle. Real HTTP token endpoints live behind this
// interface in future work; the Runtime itself never constructs a provider by default.
//
// Default assembly: unmounted. `index.ts` does not import this module, no bootstrap
// wires it, no configuration flag references it. A deployment profile with an approved
// provider adapter is required before this seam can be attached to the executor.
//
// Boundary: the provider only sees the plaintext authorization code and the handoff
// evidence that the executor already trusts. It does not learn the sealed envelope,
// Runtime private-key handle or transaction ownership; those stay inside earlier steps.

import type { OAuthCallbackHandoffClaim } from "./oauth-callback-handoff-claim-client.js";

/** Token bundle a provider returns after a successful `code` → tokens exchange. */
export interface OAuthTokenBundle {
  readonly accessToken: string;
  readonly refreshToken?: string;
  readonly tokenType: string;
  readonly expiresAt: Date;
  readonly scope?: string;
}

export interface OAuthCallbackProviderExchangeInput {
  readonly authorizationCode: string;
  readonly handoff: OAuthCallbackHandoffClaim;
}

/**
 * The three declared outcomes of a token exchange attempt. Anything a provider
 * cannot classify (network partition, unexpected exception, ambiguous 5xx) must
 * be thrown so the executor keeps the lease; only declared outcomes are safe to
 * summarise here.
 */
export type OAuthCallbackProviderExchangeOutcome =
  | Readonly<{ kind: "exchanged"; tokens: OAuthTokenBundle }>
  | Readonly<{ kind: "retryable_failure"; reason: string }>
  | Readonly<{ kind: "permanent_failure"; reason: string }>;

export interface OAuthCallbackProvider {
  exchange(input: OAuthCallbackProviderExchangeInput): Promise<OAuthCallbackProviderExchangeOutcome>;
}

/** Narrow validator used by the processor and by test doubles. */
export function isValidTokenBundle(value: OAuthTokenBundle): boolean {
  if (typeof value.accessToken !== "string" || value.accessToken.length === 0 || value.accessToken.length > 8192) return false;
  if (value.refreshToken !== undefined && (typeof value.refreshToken !== "string" || value.refreshToken.length === 0 || value.refreshToken.length > 8192)) return false;
  if (typeof value.tokenType !== "string" || value.tokenType.length === 0 || value.tokenType.length > 64) return false;
  if (!(value.expiresAt instanceof Date) || Number.isNaN(value.expiresAt.getTime())) return false;
  if (value.scope !== undefined && (typeof value.scope !== "string" || value.scope.length > 2048)) return false;
  return true;
}
