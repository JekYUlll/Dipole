// Provider-backed `OAuthCallbackHandoffProcessor`.
//
// Purpose: connect the executor to a token exchange provider and a token
// lifecycle store, translating the provider's outcome into the executor's
// existing `"completed" | "retryable_failure"` semantics without changing
// executor behaviour. Ambiguous provider failures propagate as thrown errors
// so the executor retains the lease per its existing contract.
//
// Default assembly: unmounted. `oauth-callback-handoff-runtime.ts` accepts a
// caller-supplied processor; this module is one candidate implementation. No
// bootstrap wire, no HTTP callback route, no Compose overlay.
//
// Boundary: this processor never re-opens envelopes, never renews leases and
// never calls the Core `complete/release` RPC directly — it only classifies a
// provider outcome and updates the injected lifecycle store. The executor
// still owns claim/complete/release around it.

import type { OAuthCallbackHandoffProcessor } from "./oauth-callback-handoff-executor.js";
import type { OAuthCallbackHandoffClaim } from "./oauth-callback-handoff-claim-client.js";
import type { OAuthCallbackProvider } from "./oauth-callback-provider.js";
import { TokenLifecycleStore, isTerminalOrActive, type TokenLifecycleBundle } from "./oauth-callback-token-lifecycle.js";

export interface OAuthCallbackHandoffProviderProcessorOptions {
  readonly provider: OAuthCallbackProvider;
  readonly lifecycle: TokenLifecycleStore;
  readonly persistence?: Readonly<{ persistActive(input: Readonly<{ handoff: OAuthCallbackHandoffClaim; leaseOwner: string; bundle: TokenLifecycleBundle }>): Promise<void> }>;
  readonly leaseOwner?: string;
}

export class OAuthCallbackHandoffProviderProcessor implements OAuthCallbackHandoffProcessor {
  constructor(private readonly options: OAuthCallbackHandoffProviderProcessorOptions) {}

  async process(input: Readonly<{ authorizationCode: string; handoff: OAuthCallbackHandoffClaim }>): Promise<"completed" | "retryable_failure"> {
    const { provider, lifecycle } = this.options;
    const existing = lifecycle.get(input.handoff.handoffId);
    if (existing !== undefined && isTerminalOrActive(existing)) {
      // Exact-replay guard: an earlier attempt already reached active/refreshed or a
      // permanent revocation. The executor treats this as terminal so Core's second
      // claim never succeeds; we must not invoke the provider a second time.
      return "completed";
    }
    lifecycle.markPending(input.handoff.handoffId);
    const outcome = await provider.exchange({ authorizationCode: input.authorizationCode, handoff: input.handoff });
    switch (outcome.kind) {
      case "exchanged": {
        const bundle: TokenLifecycleBundle = {
          accessToken: outcome.tokens.accessToken,
          tokenType: outcome.tokens.tokenType,
          expiresAt: outcome.tokens.expiresAt,
          ...(outcome.tokens.refreshToken !== undefined ? { refreshToken: outcome.tokens.refreshToken } : {}),
          ...(outcome.tokens.scope !== undefined ? { scope: outcome.tokens.scope } : {})
        };
        if (this.options.persistence !== undefined) {
          if (this.options.leaseOwner === undefined) throw new Error("OAuth token lifecycle lease owner is required");
          await this.options.persistence.persistActive({ handoff: input.handoff, leaseOwner: this.options.leaseOwner, bundle });
        }
        lifecycle.upsertExchange(input.handoff.handoffId, bundle);
        return "completed";
      }
      case "retryable_failure":
        return "retryable_failure";
      case "permanent_failure":
        // Record the permanent revocation so a duplicate `notifyHandoff` short-circuits
        // above; the executor still marks the handoff `exchanged` because the exchange
        // will not be attempted again.
        lifecycle.revoke(input.handoff.handoffId, outcome.reason);
        return "completed";
    }
  }
}
