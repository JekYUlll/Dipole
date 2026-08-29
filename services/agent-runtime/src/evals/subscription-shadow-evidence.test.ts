import { describe, expect, it } from "vitest";

import { createSubscriptionShadowEvidence, parseSubscriptionShadowEvidence } from "./subscription-shadow-evidence.js";

const start = { accepted_match: 10, accepted_miss: 20, accepted_error: 0, ignored_match: 5, ignored_miss: 15, ignored_error: 0, candidates: 20 };
const end = { accepted_match: 50, accepted_miss: 40, accepted_error: 0, ignored_match: 35, ignored_miss: 35, ignored_error: 0, candidates: 100 };

describe("subscription Shadow evidence", () => {
  it("creates canonical low-sensitive evidence for a complete monotonic window", () => {
    const evidence = createSubscriptionShadowEvidence(input(), { now: () => new Date("2026-08-29T01:00:00.000Z") });
    expect(evidence).toMatchObject({ outcome: "passed", observed_events: 110, counter_resets: 0, production_authority: false, runtime_change_authority: false });
    expect(parseSubscriptionShadowEvidence(evidence, { now: () => new Date("2026-08-29T02:00:00.000Z") })).toEqual(evidence);
  });

  it("rejects partial windows, insufficient scrape coverage, resets, errors, and low volume", () => {
    expect(() => createSubscriptionShadowEvidence(input({ window_end: "2026-08-28T12:00:00.000Z" }))).toThrow(/window/);
    expect(() => createSubscriptionShadowEvidence(input({ successful_scrapes: 100 }))).toThrow(/coverage/);
    expect(() => createSubscriptionShadowEvidence(input({ counter_resets: 1 }))).toThrow(/reset/);
    expect(() => createSubscriptionShadowEvidence(input({ query_revision: "subscription-shadow-v2" }))).toThrow();
    expect(() => createSubscriptionShadowEvidence(input({ end: { ...end, accepted_error: 1 } }))).toThrow(/error/);
    expect(() => createSubscriptionShadowEvidence(input({ end: { ...start, accepted_match: 11 } }))).toThrow(/volume/);
    expect(() => createSubscriptionShadowEvidence(input({ end: { ...end, ignored_miss: 1 } }))).toThrow(/monotonic/);
    expect(() => createSubscriptionShadowEvidence(input({ end: { ...end, candidates: 21 } }))).toThrow(/candidate/);
  });

  it("rejects stale and tampered evidence", () => {
    const evidence = createSubscriptionShadowEvidence(input(), { now: () => new Date("2026-08-29T01:00:00.000Z") });
    expect(() => parseSubscriptionShadowEvidence(evidence, { now: () => new Date("2026-08-30T01:00:00.000Z") })).toThrow(/invalid/);
    expect(() => parseSubscriptionShadowEvidence({ ...evidence, observed_events: 111 }, { now: () => new Date("2026-08-29T02:00:00.000Z") })).toThrow(/invalid/);
  });
});

function input(overrides: Record<string, unknown> = {}) {
  return {
    window_start: "2026-08-28T00:00:00.000Z", window_end: "2026-08-29T00:00:00.000Z",
    runtime_revision: "a".repeat(64), prometheus_config_sha256: "b".repeat(64), query_revision: "subscription-shadow-v1",
    expected_scrapes: 17_280, successful_scrapes: 17_000, counter_resets: 0, start, end, ...overrides
  };
}
