import { describe, expect, it } from "vitest";

import {
  createEventLeaseReclaimEvidence,
  parseEventLeaseReclaimEvidence
} from "./event-lease-reclaim-evidence.js";
import { runEventLeaseReclaimEvidenceCLI } from "./event-lease-reclaim-evidence-cli.js";

const now = new Date("2026-08-31T16:00:00.000Z");
const outcome = {
  event_id: "E-LEASE-RECLAIM-1",
  task_id: "TASK-LEASE-RECLAIM-1",
  expired_claim_reclaimed: true,
  stale_owner_completion_rejected: true,
  reclaim_attempt_count: 2,
  completed_event_count: 1
} as const;

describe("Event lease reclaim evidence", () => {
  it("binds a MySQL lease-expiry reclaim drill to one final event completion", () => {
    const evidence = createEventLeaseReclaimEvidence(outcome, { now: () => now });
    expect(evidence).toMatchObject({ outcome: "passed", production_authority: false, content_sha256: expect.stringMatching(/^[a-f0-9]{64}$/) });
    expect(parseEventLeaseReclaimEvidence(evidence, { now: () => now })).toEqual(evidence);
  });

  it("rejects a stale-owner or final-completion count drift", () => {
    const evidence = createEventLeaseReclaimEvidence(outcome, { now: () => now });
    expect(() => parseEventLeaseReclaimEvidence({ ...evidence, stale_owner_completion_rejected: false }, { now: () => now })).toThrow();
    expect(() => parseEventLeaseReclaimEvidence({ ...evidence, completed_event_count: 2 }, { now: () => now })).toThrow();
  });

  it("rejects expired evidence", () => {
    const evidence = createEventLeaseReclaimEvidence(outcome, { now: () => now, validityMs: 1_000 });
    expect(() => parseEventLeaseReclaimEvidence(evidence, { now: () => new Date("2026-08-31T16:00:02.000Z") })).toThrow(/invalid/i);
  });

  it("checks exactly one receipt path through the CLI", async () => {
    const output: string[] = [];
    const errors: string[] = [];
    await expect(runEventLeaseReclaimEvidenceCLI([], writer(output), writer(errors), () => now)).resolves.toBe(1);
    expect(errors).toEqual(["Event lease reclaim evidence check requires exactly one --evidence=<path> argument\n"]);
  });
});

function writer(values: string[]) {
  return { write(value: string) { values.push(value); } };
}
