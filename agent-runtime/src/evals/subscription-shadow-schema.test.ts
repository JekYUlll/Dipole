import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";

import { createSubscriptionShadowEvidence, subscriptionShadowEvidenceSchemaVersion } from "./subscription-shadow-evidence.js";

describe("subscription Shadow language-neutral schemas", () => {
  it("keeps evidence version and required fields aligned with TypeScript output", async () => {
    const schema = JSON.parse(await readFile(new URL("../../../contracts/agent-subscription-shadow/v1/evidence.schema.json", import.meta.url), "utf8"));
    const evidence = createSubscriptionShadowEvidence(input(), { now: () => new Date("2026-08-29T01:00:00.000Z") });
    expect(schema["x-dipole-version"]).toBe(subscriptionShadowEvidenceSchemaVersion);
    expect([...schema.required].sort()).toEqual(Object.keys(evidence).sort());
    expect(schema.additionalProperties).toBe(false);
    expect(schema.properties.query_revision.const).toBe("subscription-shadow-v1");
    expect(schema.properties.observed_events.maximum).toBe(Number.MAX_SAFE_INTEGER);
  });

  it("keeps the Prometheus snapshot input schema exact", async () => {
    const schema = JSON.parse(await readFile(new URL("../../../contracts/agent-subscription-shadow/v1/input.schema.json", import.meta.url), "utf8"));
    expect([...schema.required].sort()).toEqual(Object.keys(input()).sort());
    expect(schema.additionalProperties).toBe(false);
    expect(schema.$defs.counters.additionalProperties).toBe(false);
    expect(schema.properties.query_revision.const).toBe("subscription-shadow-v1");
    expect(schema.$defs.counters.properties.candidates.maximum).toBe(Number.MAX_SAFE_INTEGER);
  });
});

function input() { return {
  window_start: "2026-08-28T00:00:00.000Z", window_end: "2026-08-29T00:00:00.000Z",
  runtime_revision: "a".repeat(64), prometheus_config_sha256: "b".repeat(64), query_revision: "subscription-shadow-v1",
  expected_scrapes: 100, successful_scrapes: 100, counter_resets: 0,
  start: { accepted_match: 0, accepted_miss: 0, accepted_error: 0, ignored_match: 0, ignored_miss: 0, ignored_error: 0, candidates: 0 },
  end: { accepted_match: 25, accepted_miss: 25, accepted_error: 0, ignored_match: 25, ignored_miss: 25, ignored_error: 0, candidates: 50 }
}; }
