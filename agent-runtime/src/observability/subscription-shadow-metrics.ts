export type SubscriptionShadowOutcome = "match" | "miss" | "error";

export interface SubscriptionShadowObservation {
  readonly directTargetAccepted: boolean;
  readonly subscriptionOutcome: SubscriptionShadowOutcome;
  readonly candidateCount: number;
}

export interface SubscriptionShadowObserver {
  observe(observation: SubscriptionShadowObservation): void;
}

const directTargetStates = ["accepted", "ignored"] as const;
const subscriptionStates = ["match", "miss", "error"] as const;

export class SubscriptionShadowMetrics implements SubscriptionShadowObserver {
  readonly #comparisons = new Map<string, number>();
  #candidates = 0;

  constructor(readonly enabled = true) {}

  observe(observation: SubscriptionShadowObservation): void {
    if (!subscriptionStates.includes(observation.subscriptionOutcome)) {
      throw new Error("Subscription Shadow outcome is invalid");
    }
    if (!Number.isSafeInteger(observation.candidateCount) || observation.candidateCount < 0) {
      throw new Error("Subscription Shadow candidate count must be a non-negative integer");
    }
    const directTarget = observation.directTargetAccepted ? "accepted" : "ignored";
    const key = `${directTarget}:${observation.subscriptionOutcome}`;
    this.#comparisons.set(key, (this.#comparisons.get(key) ?? 0) + 1);
    this.#candidates += observation.candidateCount;
  }

  render(): string {
    const lines = [
      "# HELP dipole_agent_subscription_shadow_enabled Whether deterministic subscription Shadow observation is enabled.",
      "# TYPE dipole_agent_subscription_shadow_enabled gauge",
      `dipole_agent_subscription_shadow_enabled ${this.enabled ? 1 : 0}`,
      "# HELP dipole_agent_subscription_shadow_comparisons_total Direct-target and deterministic subscription Shadow outcomes.",
      "# TYPE dipole_agent_subscription_shadow_comparisons_total counter"
    ];
    for (const directTarget of directTargetStates) {
      for (const subscription of subscriptionStates) {
        lines.push(`dipole_agent_subscription_shadow_comparisons_total{direct_target="${directTarget}",subscription="${subscription}"} ${this.#comparisons.get(`${directTarget}:${subscription}`) ?? 0}`);
      }
    }
    lines.push(
      "# HELP dipole_agent_subscription_shadow_candidates_total Deterministic subscription candidates observed without admission.",
      "# TYPE dipole_agent_subscription_shadow_candidates_total counter",
      `dipole_agent_subscription_shadow_candidates_total ${this.#candidates}`,
      ""
    );
    return lines.join("\n");
  }
}
