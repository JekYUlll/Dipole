// Deterministic fake `OAuthCallbackProvider` for offline composition tests.
//
// Purpose: exercise the executor + processor + lifecycle chain without any real
// HTTP token endpoint. Callers pre-declare an outcome per authorization code;
// the fake returns the same outcome for repeat exchanges of the same code so
// duplicate `notifyHandoff` cannot yield a second issuance.
//
// Default assembly: unmounted; only test files import this module. Not exported
// from any bootstrap and not selected by any environment variable.
//
// Boundary: it never opens an envelope, never talks to a Runtime private key,
// never touches the token lifecycle store. It only classifies a plaintext code
// as `exchanged | retryable_failure | permanent_failure` or defers via a thrown
// error to model an ambiguous provider outcome.

import { createHash } from "node:crypto";

import type {
  OAuthCallbackProvider,
  OAuthCallbackProviderExchangeInput,
  OAuthCallbackProviderExchangeOutcome
} from "./oauth-callback-provider.js";

export type DeterministicOutcome = OAuthCallbackProviderExchangeOutcome | Readonly<{ kind: "throw"; reason: string }>;

export interface DeterministicFakeOAuthCallbackProviderConfig {
  /** Map of `sha256(authorizationCode)` → outcome. */
  readonly plan: ReadonlyMap<string, DeterministicOutcome>;
  /** Optional default outcome when the code is not in the plan. Defaults to permanent failure. */
  readonly defaultOutcome?: DeterministicOutcome;
}

export class DeterministicFakeOAuthCallbackProvider implements OAuthCallbackProvider {
  private readonly plan: ReadonlyMap<string, DeterministicOutcome>;
  private readonly defaultOutcome: DeterministicOutcome;
  private readonly cache: Map<string, OAuthCallbackProviderExchangeOutcome> = new Map();
  private readonly counts: Map<string, number> = new Map();

  constructor(config: DeterministicFakeOAuthCallbackProviderConfig) {
    this.plan = config.plan;
    this.defaultOutcome = config.defaultOutcome ?? Object.freeze({ kind: "permanent_failure", reason: "no plan entry" });
  }

  async exchange(input: OAuthCallbackProviderExchangeInput): Promise<OAuthCallbackProviderExchangeOutcome> {
    const digest = createHash("sha256").update(input.authorizationCode).digest("hex");
    this.counts.set(digest, (this.counts.get(digest) ?? 0) + 1);
    const cached = this.cache.get(digest);
    if (cached !== undefined) return cached;
    const outcome = this.plan.get(digest) ?? this.defaultOutcome;
    if (outcome.kind === "throw") throw new Error(outcome.reason);
    // Only cache terminal-from-provider outcomes so a duplicate call cannot yield
    // a second token issuance for the same code. Retryable failures deliberately
    // leave no cached entry: the caller can change the plan between attempts to
    // model an intermittent timeout that eventually succeeds.
    if (outcome.kind === "exchanged" || outcome.kind === "permanent_failure") {
      this.cache.set(digest, outcome);
    }
    return outcome;
  }

  exchangeCount(authorizationCode: string): number {
    return this.counts.get(createHash("sha256").update(authorizationCode).digest("hex")) ?? 0;
  }
}
