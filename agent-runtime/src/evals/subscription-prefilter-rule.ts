import { createHash } from "node:crypto";

import { matchEventSubscriptions, parseAgentEventSubscription, type AgentEventSubscription } from "../events/event-subscription.js";
import { canonicalJSON } from "./offline-evaluator.js";
import {
  parseSubscriptionPrefilterEvidence,
  type SubscriptionPrefilterCorpus,
  type SubscriptionPrefilterEvidence
} from "./subscription-prefilter-evaluator.js";

export interface RulePrefilterMeasurement {
  readonly nowMicros: () => number;
}

export function buildRulePrefilterEvidence(
  corpus: SubscriptionPrefilterCorpus,
  subscription: AgentEventSubscription,
  measurement: RulePrefilterMeasurement = { nowMicros: defaultNowMicros }
): SubscriptionPrefilterEvidence {
  const validatedSubscription = parseAgentEventSubscription(subscription);
  const configurationSha256 = createHash("sha256").update(canonicalJSON(validatedSubscription)).digest("hex");
  const decisions = corpus.cases.map(testCase => {
    const started = measurement.nowMicros();
    const selected = matchEventSubscriptions(testCase.event, [validatedSubscription]).length === 1;
    const finished = measurement.nowMicros();
    return {
      caseId: testCase.id,
      selected,
      latencyMicros: Math.max(0, Math.ceil(finished - started)),
      costMicrousd: 0
    };
  });
  return parseSubscriptionPrefilterEvidence({
    schemaVersion: "dipole.agent.subscription-prefilter-evidence.v1",
    corpusSha256: corpus.sha256,
    candidate: {
      id: `rule:${configurationSha256}`,
      kind: "rule",
      revision: `subscription:${configurationSha256}`,
      configurationSha256
    },
    decisions
  });
}

function defaultNowMicros(): number {
  return performance.now() * 1000;
}
