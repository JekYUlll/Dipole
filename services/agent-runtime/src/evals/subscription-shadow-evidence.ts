import { createHash } from "node:crypto";
import { z } from "zod";

import { canonicalJSON } from "./offline-evaluator.js";

export const subscriptionShadowEvidenceSchemaVersion = "dipole.agent.subscription-shadow-evidence.v1" as const;
export const subscriptionShadowQueryRevision = "subscription-shadow-v1" as const;
const DAY = 24 * 60 * 60 * 1_000;
const sha = z.string().regex(/^[a-f0-9]{64}$/);
const count = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const counters = z.object({
  accepted_match: count, accepted_miss: count, accepted_error: count,
  ignored_match: count, ignored_miss: count, ignored_error: count, candidates: count
}).strict();
const inputSchema = z.object({
  window_start: z.string(), window_end: z.string(), runtime_revision: sha,
  prometheus_config_sha256: sha, query_revision: z.literal(subscriptionShadowQueryRevision),
  expected_scrapes: count.positive(), successful_scrapes: count, counter_resets: count,
  start: counters, end: counters
}).strict();
const deltasSchema = counters.omit({ candidates: true });
const payloadSchema = z.object({
  schema_version: z.literal(subscriptionShadowEvidenceSchemaVersion), outcome: z.literal("passed"),
  window_start: z.string(), window_end: z.string(), collected_at: z.string(), expires_at: z.string(),
  runtime_revision: sha, prometheus_config_sha256: sha, query_revision: z.literal(subscriptionShadowQueryRevision),
  expected_scrapes: count.positive(), successful_scrapes: count, counter_resets: z.literal(0),
  deltas: deltasSchema, candidate_delta: count, observed_events: count,
  production_authority: z.literal(false), runtime_change_authority: z.literal(false)
}).strict();
const evidenceSchema = payloadSchema.extend({ content_sha256: sha }).strict();

export type SubscriptionShadowEvidence = z.infer<typeof evidenceSchema>;
export type SubscriptionShadowEvidenceInput = z.infer<typeof inputSchema>;
type Clock = { readonly now?: () => Date };

export function createSubscriptionShadowEvidence(value: unknown, options: Clock = {}): SubscriptionShadowEvidence {
  const input = inputSchema.parse(value);
  const windowStart = canonicalDate(input.window_start), windowEnd = canonicalDate(input.window_end);
  const duration = windowEnd.getTime() - windowStart.getTime();
  if (duration < DAY || duration > 31 * DAY) throw new Error("Subscription Shadow evidence window is invalid");
  if (input.successful_scrapes > input.expected_scrapes || input.successful_scrapes * 100 < input.expected_scrapes * 95) {
    throw new Error("Subscription Shadow evidence scrape coverage is insufficient");
  }
  if (input.counter_resets !== 0) throw new Error("Subscription Shadow evidence has a counter reset");
  const keys = Object.keys(input.start) as Array<keyof typeof input.start>;
  if (keys.some(key => input.end[key] < input.start[key])) throw new Error("Subscription Shadow counters are not monotonic");
  const delta = Object.fromEntries(keys.map(key => [key, input.end[key] - input.start[key]])) as typeof input.start;
  const observedEvents = delta.accepted_match + delta.accepted_miss + delta.accepted_error + delta.ignored_match + delta.ignored_miss + delta.ignored_error;
  if (observedEvents < 100) throw new Error("Subscription Shadow evidence volume is insufficient");
  if (delta.accepted_error + delta.ignored_error !== 0) throw new Error("Subscription Shadow evidence contains matcher errors");
  if (delta.candidates < delta.accepted_match + delta.ignored_match) throw new Error("Subscription Shadow candidate count is inconsistent");
  const collectedAt = validDate((options.now ?? (() => new Date()))());
  if (collectedAt.getTime() < windowEnd.getTime()) throw new Error("Subscription Shadow evidence collection time is invalid");
  const payload = payloadSchema.parse({
    schema_version: subscriptionShadowEvidenceSchemaVersion, outcome: "passed",
    window_start: input.window_start, window_end: input.window_end, collected_at: collectedAt.toISOString(),
    expires_at: new Date(collectedAt.getTime() + DAY).toISOString(), runtime_revision: input.runtime_revision,
    prometheus_config_sha256: input.prometheus_config_sha256, query_revision: input.query_revision,
    expected_scrapes: input.expected_scrapes, successful_scrapes: input.successful_scrapes, counter_resets: 0,
    deltas: { accepted_match: delta.accepted_match, accepted_miss: delta.accepted_miss, accepted_error: 0,
      ignored_match: delta.ignored_match, ignored_miss: delta.ignored_miss, ignored_error: 0 },
    candidate_delta: delta.candidates, observed_events: observedEvents,
    production_authority: false, runtime_change_authority: false
  });
  return Object.freeze({ ...payload, content_sha256: digest(payload) });
}

export function parseSubscriptionShadowEvidence(value: unknown, options: Clock = {}): SubscriptionShadowEvidence {
  const evidence = evidenceSchema.parse(value);
  const start = canonicalDate(evidence.window_start), end = canonicalDate(evidence.window_end);
  const collected = canonicalDate(evidence.collected_at), expires = canonicalDate(evidence.expires_at);
  const now = validDate((options.now ?? (() => new Date()))());
  const { content_sha256, ...payload } = evidence;
  const total = Object.values(evidence.deltas).reduce((sum, item) => sum + item, 0);
  const duration = end.getTime() - start.getTime();
  if (duration < DAY || duration > 31 * DAY || collected < end || expires.getTime() - collected.getTime() !== DAY ||
      now < collected || now >= expires || evidence.observed_events !== total || evidence.observed_events < 100 ||
      evidence.successful_scrapes > evidence.expected_scrapes || evidence.successful_scrapes * 100 < evidence.expected_scrapes * 95 ||
      evidence.deltas.accepted_error !== 0 || evidence.deltas.ignored_error !== 0 ||
      evidence.candidate_delta < evidence.deltas.accepted_match + evidence.deltas.ignored_match || digest(payload) !== content_sha256) {
    throw new Error("Subscription Shadow evidence is invalid");
  }
  return Object.freeze(evidence);
}

function digest(payload: z.infer<typeof payloadSchema>): string {
  return createHash("sha256").update(`${subscriptionShadowEvidenceSchemaVersion}\n${canonicalJSON(payload)}`, "utf8").digest("hex");
}
function canonicalDate(raw: string): Date { const value = validDate(new Date(raw)); if (value.toISOString() !== raw) throw new Error("Subscription Shadow evidence time is invalid"); return value; }
function validDate(value: Date): Date { if (!Number.isFinite(value.getTime())) throw new Error("Subscription Shadow evidence time is invalid"); return value; }
