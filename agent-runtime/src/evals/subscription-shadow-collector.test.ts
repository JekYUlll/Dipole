import { describe, expect, it, vi } from "vitest";

import {
  HTTPPrometheusQueryClient,
  buildSubscriptionShadowCollectionQueries,
  collectSubscriptionShadowEvidenceInput
} from "./subscription-shadow-collector.js";
import { createSubscriptionShadowEvidence } from "./subscription-shadow-evidence.js";

const windowStart = "2026-08-28T00:00:00.000Z";
const windowEnd = "2026-08-29T00:00:00.000Z";

describe("subscription Shadow Prometheus collector", () => {
  it("collects the exact single-series window into evidence v1 input", async () => {
    const queries = buildSubscriptionShadowCollectionQueries(86_400);
    const values = cleanValues(queries);
    const calls: Array<{ expression: string; at: string }> = [];
    const input = await collectSubscriptionShadowEvidenceInput(request(), {
      async query(expression, at) { calls.push({ expression, at }); return vector(values.get(sampleKey(expression, at))); }
    });

    expect(calls).toHaveLength(19);
    expect(new Set(calls.map(call => call.expression)).size).toBe(12);
    expect(input).toEqual({
      window_start: windowStart, window_end: windowEnd,
      runtime_revision: "a".repeat(64), prometheus_config_sha256: "b".repeat(64),
      query_revision: "subscription-shadow-v1", expected_scrapes: 17_280,
      successful_scrapes: 17_200, counter_resets: 0,
      start: counters(0),
      end: { accepted_match: 25, accepted_miss: 25, accepted_error: 0, ignored_match: 25, ignored_miss: 25, ignored_error: 0, candidates: 50 }
    });
    expect(calls.filter(call => call.at === windowStart)).toHaveLength(7);
    expect(calls.filter(call => call.at === windowEnd)).toHaveLength(12);
    expect(createSubscriptionShadowEvidence(input, { now: () => new Date("2026-08-29T01:00:00.000Z") })).toMatchObject({
      outcome: "passed", observed_events: 100, candidate_delta: 50,
      production_authority: false, runtime_change_authority: false
    });
  });

  it("fails closed for disabled windows, missing or multiple series, and unsafe values", async () => {
    const queries = buildSubscriptionShadowCollectionQueries(86_400);
    const values = cleanValues(queries);
    for (const response of [vector(undefined), multiVector(1), vector("NaN"), vector("1.5")]) {
      const client = { query: vi.fn(async (expression: string, at: string) => expression === queries.start.accepted_match && at === windowStart ? response : vector(values.get(sampleKey(expression, at)))) };
      await expect(collectSubscriptionShadowEvidenceInput(request(), client)).rejects.toThrow(/invalid/);
    }
    values.set(sampleKey(queries.enabledMinimum, windowEnd), 0);
    await expect(collectSubscriptionShadowEvidenceInput(request(), { query: async (expression, at) => vector(values.get(sampleKey(expression, at))) })).rejects.toThrow(/enabled/);
    await expect(collectSubscriptionShadowEvidenceInput(request(), { query: async () => ({ status: "error", error: "sensitive upstream detail" }) })).rejects.toThrow(/invalid/);
  });

  it("validates the explicit URL and fixed Prometheus response envelope", async () => {
    expect(() => new HTTPPrometheusQueryClient("https://user:secret@prometheus.example")).toThrow(/URL/);
    expect(() => new HTTPPrometheusQueryClient("https://prometheus.example/api?token=x")).toThrow(/URL/);
    const fetcher = vi.fn(async (input: string | URL | Request) => {
      const url = new URL(input.toString());
      expect(url.pathname).toBe("/api/v1/query");
      expect(url.searchParams.get("query")).toBe("up");
      expect(url.searchParams.get("time")).toBe(windowEnd);
      return new Response(JSON.stringify(vector(1)), { status: 200, headers: { "content-type": "application/json" } });
    });
    const client = new HTTPPrometheusQueryClient("https://prometheus.example", fetcher as typeof fetch);
    await expect(client.query("up", windowEnd)).resolves.toEqual(vector(1));
  });
});

function request() { return {
  schema_version: "dipole.agent.subscription-shadow-collection.v1",
  prometheus_url: "http://prometheus:9090", window_start: windowStart, window_end: windowEnd,
  runtime_revision: "a".repeat(64), prometheus_config_sha256: "b".repeat(64), scrape_interval_seconds: 5
}; }

function cleanValues(queries: ReturnType<typeof buildSubscriptionShadowCollectionQueries>) {
  const values = new Map<string, number>();
  for (const expression of Object.values(queries.start)) values.set(sampleKey(expression, windowStart), 0);
  Object.entries(queries.end).forEach(([key, expression]) => values.set(sampleKey(expression, windowEnd), key === "candidates" ? 50 : key.endsWith("match") || key.endsWith("miss") ? 25 : 0));
  values.set(sampleKey(queries.successfulScrapes, windowEnd), 17_200);
  values.set(sampleKey(queries.comparisonResets, windowEnd), 0);
  values.set(sampleKey(queries.candidateResets, windowEnd), 0);
  values.set(sampleKey(queries.enabledMinimum, windowEnd), 1);
  values.set(sampleKey(queries.enabledMaximum, windowEnd), 1);
  return values;
}

function counters(value: number) { return {
  accepted_match: value, accepted_miss: value, accepted_error: value,
  ignored_match: value, ignored_miss: value, ignored_error: value, candidates: value
}; }
function vector(value: number | string | undefined) { return {
  status: "success", data: { resultType: "vector", result: value === undefined ? [] : [{ metric: {}, value: [1_777_593_600, String(value)] }] }
}; }
function multiVector(value: number) { const response = vector(value); response.data.result.push({ metric: { instance: "second" }, value: [1_777_593_600, String(value)] }); return response; }
function sampleKey(expression: string, at: string) { return `${at}\n${expression}`; }
