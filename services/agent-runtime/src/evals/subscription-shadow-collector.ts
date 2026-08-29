import { z } from "zod";

import {
  subscriptionShadowQueryRevision,
  type SubscriptionShadowEvidenceInput
} from "./subscription-shadow-evidence.js";

export const subscriptionShadowCollectionSchemaVersion = "dipole.agent.subscription-shadow-collection.v1" as const;
const maxPrometheusResponseBytes = 256 * 1024;
const sha = z.string().regex(/^[a-f0-9]{64}$/);
const requestSchema = z.object({
  schema_version: z.literal(subscriptionShadowCollectionSchemaVersion),
  prometheus_url: z.string().trim().min(1).max(2_048),
  window_start: z.string(), window_end: z.string(),
  runtime_revision: sha, prometheus_config_sha256: sha,
  scrape_interval_seconds: z.number().int().min(1).max(3_600)
}).strict();
type CollectionRequest = z.infer<typeof requestSchema>;
type CounterKey = keyof SubscriptionShadowEvidenceInput["start"];

export interface PrometheusQueryClient {
  query(expression: string, at: string): Promise<unknown>;
}

export class HTTPPrometheusQueryClient implements PrometheusQueryClient {
  readonly #baseURL: string;
  readonly #fetch: typeof fetch;

  constructor(baseURL: string, fetcher: typeof fetch = fetch) {
    this.#baseURL = canonicalPrometheusURL(baseURL);
    this.#fetch = fetcher;
  }

  async query(expression: string, at: string): Promise<unknown> {
    const url = new URL("/api/v1/query", this.#baseURL);
    url.searchParams.set("query", expression);
    url.searchParams.set("time", at);
    let response: Response;
    try {
      response = await this.#fetch(url, { headers: { accept: "application/json" }, signal: AbortSignal.timeout(10_000) });
    } catch {
      throw new Error("Subscription Shadow Prometheus query failed");
    }
    if (!response.ok) throw new Error("Subscription Shadow Prometheus query failed");
    try {
      const body = await readBoundedBody(response, maxPrometheusResponseBytes);
      return JSON.parse(body) as unknown;
    }
    catch { throw new Error("Subscription Shadow Prometheus response is invalid"); }
  }
}

export function buildSubscriptionShadowCollectionQueries(windowSeconds: number) {
  if (!Number.isSafeInteger(windowSeconds) || windowSeconds < 1) throw new Error("Subscription Shadow collection window is invalid");
  const comparisons = (directTarget: "accepted" | "ignored", subscription: "match" | "miss" | "error") =>
    `dipole_agent_subscription_shadow_comparisons_total{job="dipole-agent",direct_target="${directTarget}",subscription="${subscription}"}`;
  const counters = Object.freeze({
    accepted_match: comparisons("accepted", "match"), accepted_miss: comparisons("accepted", "miss"),
    accepted_error: comparisons("accepted", "error"), ignored_match: comparisons("ignored", "match"),
    ignored_miss: comparisons("ignored", "miss"), ignored_error: comparisons("ignored", "error"),
    candidates: 'dipole_agent_subscription_shadow_candidates_total{job="dipole-agent"}'
  });
  const range = `${windowSeconds}s`;
  return Object.freeze({
    start: counters, end: counters,
    successfulScrapes: `count_over_time(up{job="dipole-agent"}[${range}])`,
    comparisonResets: `sum(resets(dipole_agent_subscription_shadow_comparisons_total{job="dipole-agent"}[${range}]))`,
    candidateResets: `sum(resets(dipole_agent_subscription_shadow_candidates_total{job="dipole-agent"}[${range}]))`,
    enabledMinimum: `min_over_time(dipole_agent_subscription_shadow_enabled{job="dipole-agent"}[${range}])`,
    enabledMaximum: `max_over_time(dipole_agent_subscription_shadow_enabled{job="dipole-agent"}[${range}])`
  });
}

export async function collectSubscriptionShadowEvidenceInput(
  value: unknown, client: PrometheusQueryClient
): Promise<SubscriptionShadowEvidenceInput> {
  const request = parseRequest(value);
  const start = canonicalDate(request.window_start), end = canonicalDate(request.window_end);
  const windowSeconds = (end.getTime() - start.getTime()) / 1_000;
  if (!Number.isSafeInteger(windowSeconds) || windowSeconds < 86_400 || windowSeconds > 31 * 86_400 ||
      windowSeconds % request.scrape_interval_seconds !== 0) {
    throw new Error("Subscription Shadow collection window is invalid");
  }
  const queries = buildSubscriptionShadowCollectionQueries(windowSeconds);
  const startCounters = await queryCounters(client, queries.start, request.window_start);
  const endCounters = await queryCounters(client, queries.end, request.window_end);
  const [successfulScrapes, comparisonResets, candidateResets, enabledMinimum, enabledMaximum] = await Promise.all([
    queryInteger(client, queries.successfulScrapes, request.window_end),
    queryInteger(client, queries.comparisonResets, request.window_end),
    queryInteger(client, queries.candidateResets, request.window_end),
    queryInteger(client, queries.enabledMinimum, request.window_end),
    queryInteger(client, queries.enabledMaximum, request.window_end)
  ]);
  if (enabledMinimum !== 1 || enabledMaximum !== 1) throw new Error("Subscription Shadow collection window was not continuously enabled");
  const counterResets = comparisonResets + candidateResets;
  if (!Number.isSafeInteger(counterResets)) throw new Error("Subscription Shadow Prometheus response is invalid");
  return Object.freeze({
    window_start: request.window_start, window_end: request.window_end,
    runtime_revision: request.runtime_revision, prometheus_config_sha256: request.prometheus_config_sha256,
    query_revision: subscriptionShadowQueryRevision,
    expected_scrapes: windowSeconds / request.scrape_interval_seconds,
    successful_scrapes: successfulScrapes, counter_resets: counterResets,
    start: startCounters, end: endCounters
  });
}

export async function collectSubscriptionShadowEvidenceInputFromPrometheus(value: unknown): Promise<SubscriptionShadowEvidenceInput> {
  const request = parseRequest(value);
  return collectSubscriptionShadowEvidenceInput(request, new HTTPPrometheusQueryClient(request.prometheus_url));
}

async function queryCounters(client: PrometheusQueryClient, queries: Readonly<Record<CounterKey, string>>, at: string) {
  const entries = await Promise.all(Object.entries(queries).map(async ([key, expression]) => [key, await queryInteger(client, expression, at)] as const));
  return Object.freeze(Object.fromEntries(entries)) as SubscriptionShadowEvidenceInput["start"];
}

async function queryInteger(client: PrometheusQueryClient, expression: string, at: string): Promise<number> {
  const value = await client.query(expression, at);
  if (!isRecord(value) || value.status !== "success" || !isRecord(value.data) || value.data.resultType !== "vector" ||
      !Array.isArray(value.data.result) || value.data.result.length !== 1) {
    throw new Error("Subscription Shadow Prometheus response is invalid");
  }
  const sample = value.data.result[0];
  if (!isRecord(sample) || !Array.isArray(sample.value) || sample.value.length !== 2 || typeof sample.value[1] !== "string") {
    throw new Error("Subscription Shadow Prometheus response is invalid");
  }
  const parsed = Number(sample.value[1]);
  if (!Number.isSafeInteger(parsed) || parsed < 0) throw new Error("Subscription Shadow Prometheus response is invalid");
  return parsed;
}

function parseRequest(value: unknown): CollectionRequest {
  const request = requestSchema.parse(value);
  canonicalPrometheusURL(request.prometheus_url);
  return request;
}
function canonicalPrometheusURL(raw: string): string {
  let url: URL;
  try { url = new URL(raw); } catch { throw new Error("Subscription Shadow Prometheus URL is invalid"); }
  if ((url.protocol !== "http:" && url.protocol !== "https:") || url.username || url.password || url.search || url.hash ||
      (url.pathname !== "/" && url.pathname !== "")) throw new Error("Subscription Shadow Prometheus URL is invalid");
  return url.origin;
}
function canonicalDate(raw: string): Date {
  const value = new Date(raw);
  if (!Number.isFinite(value.getTime()) || value.toISOString() !== raw) throw new Error("Subscription Shadow collection time is invalid");
  return value;
}
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }

async function readBoundedBody(response: Response, maximumBytes: number): Promise<string> {
  const declaredLength = response.headers.get("content-length");
  if (declaredLength !== null && (!/^\d+$/u.test(declaredLength) || Number(declaredLength) > maximumBytes)) {
    throw new Error("response body is too large");
  }
  if (response.body === null) {
    const body = await response.text();
    if (new TextEncoder().encode(body).byteLength > maximumBytes) throw new Error("response body is too large");
    return body;
  }
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  try {
    while (true) {
      const next = await reader.read();
      if (next.done) break;
      total += next.value.byteLength;
      if (total > maximumBytes) throw new Error("response body is too large");
      chunks.push(next.value);
    }
  } catch (error) {
    await reader.cancel().catch(() => undefined);
    throw error;
  } finally {
    reader.releaseLock();
  }
  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
  return new TextDecoder().decode(bytes);
}
