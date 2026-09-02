import { describe, expect, it } from "vitest";

import { SubscriptionShadowMetrics } from "./subscription-shadow-metrics.js";

describe("SubscriptionShadowMetrics", () => {
  it("renders a bounded comparison matrix without event or authority identifiers", () => {
    const metrics = new SubscriptionShadowMetrics();
    metrics.observe({ directTargetAccepted: true, subscriptionOutcome: "match", candidateCount: 2 });
    metrics.observe({ directTargetAccepted: false, subscriptionOutcome: "miss", candidateCount: 0 });
    metrics.observe({ directTargetAccepted: true, subscriptionOutcome: "error", candidateCount: 0 });

    const output = metrics.render();
    expect(output).toContain('dipole_agent_subscription_shadow_comparisons_total{direct_target="accepted",subscription="match"} 1');
    expect(output).toContain('dipole_agent_subscription_shadow_comparisons_total{direct_target="ignored",subscription="miss"} 1');
    expect(output).toContain('dipole_agent_subscription_shadow_comparisons_total{direct_target="accepted",subscription="error"} 1');
    expect(output).toContain("dipole_agent_subscription_shadow_candidates_total 2");
    expect(output).toContain("dipole_agent_subscription_shadow_enabled 1");
    expect(output).not.toMatch(/event|principal|definition|subscription_id/i);
  });

  it("exposes disabled state with zero counters", () => {
    const output = new SubscriptionShadowMetrics(false).render();
    expect(output).toContain("dipole_agent_subscription_shadow_enabled 0");
    expect(output).toContain("dipole_agent_subscription_shadow_candidates_total 0");
  });

  it("rejects runtime outcomes outside the bounded metric vocabulary", () => {
    const metrics = new SubscriptionShadowMetrics();
    expect(() => metrics.observe({ directTargetAccepted: true, subscriptionOutcome: "overflow" as "match", candidateCount: 0 }))
      .toThrow(/outcome is invalid/);
  });
});
