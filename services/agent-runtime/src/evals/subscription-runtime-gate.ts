import { createHash } from "node:crypto";

import { canonicalJSON } from "./offline-evaluator.js";
import type { SubscriptionRolloutDecision } from "./subscription-rollout-evaluator.js";

export type SubscriptionRuntimeMode = "off" | "shadow" | "enforced";

export interface SubscriptionRuntimeBinding {
  readonly mode: SubscriptionRuntimeMode;
  readonly decision: SubscriptionRolloutDecision;
  readonly decisionSha256?: string;
  readonly candidateId?: string;
  readonly configurationSha256?: string;
  readonly corpusSha256?: string;
  readonly reviewSha256?: string;
  readonly finalLabelsSha256?: string;
  readonly candidateEvidenceSha256?: string;
}

export interface SubscriptionRuntimeResult {
  readonly mode: SubscriptionRuntimeMode;
  readonly outcome: "bypassed" | "observed" | "admitted" | "blocked";
  readonly taskCreationAllowed: boolean;
  readonly reason: string;
}

export class SubscriptionRuntimeGate {
  constructor(private readonly binding: SubscriptionRuntimeBinding) {}

  evaluate(): SubscriptionRuntimeResult {
    if (this.binding.mode === "off") return this.result("bypassed", true, "subscription_prefilter_disabled");
    this.verifyBinding();
    if (this.binding.mode === "shadow") return this.result("observed", true, "subscription_prefilter_shadow_only");
    if (this.binding.decision.decision !== "eligible") return this.result("blocked", false, "subscription_rollout_blocked");
    return this.result("admitted", true, "subscription_rollout_eligible");
  }

  private verifyBinding(): void {
    const { decision } = this.binding;
    if (this.binding.decisionSha256 !== digest(decision)) throw new Error("subscription rollout decision hash drift");
    if (decision.candidate.id !== this.binding.candidateId || decision.candidate.configurationSha256 !== this.binding.configurationSha256) {
      throw new Error("subscription prefilter candidate binding drift");
    }
    if (decision.corpusSha256 !== this.binding.corpusSha256 || decision.reviewSha256 !== this.binding.reviewSha256 ||
      decision.finalLabelsSha256 !== this.binding.finalLabelsSha256 || decision.candidateEvidenceSha256 !== this.binding.candidateEvidenceSha256) {
      throw new Error("subscription prefilter evidence binding drift");
    }
  }

  private result(outcome: SubscriptionRuntimeResult["outcome"], taskCreationAllowed: boolean, reason: string): SubscriptionRuntimeResult {
    return { mode: this.binding.mode, outcome, taskCreationAllowed, reason };
  }
}

function digest(value: unknown): string { return createHash("sha256").update(canonicalJSON(value)).digest("hex"); }
