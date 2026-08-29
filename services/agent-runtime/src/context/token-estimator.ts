import { createHash } from "node:crypto";

import { z } from "zod";

export const routeContextProfileSchema = z.object({
  route: z.string().trim().min(1),
  contextWindowTokens: z.number().int().min(1_024).max(10_000_000),
  utf8BytesPerToken: z.number().min(1).max(16),
  safetyMarginBps: z.number().int().min(0).max(10_000)
}).strict();

const routeContextProfilesSchema = z.array(routeContextProfileSchema).refine(
  (profiles) => new Set(profiles.map((profile) => profile.route)).size === profiles.length,
  "Model route context profiles must be unique"
);

export type RouteContextProfile = z.infer<typeof routeContextProfileSchema>;

export interface RouteTokenEstimator {
  readonly id: string;
  readonly contextWindowTokens: number;
  readonly fallbackRoutes: readonly string[];
  estimate(text: string): number;
}

export interface RouteTokenizerAdapter {
  readonly route: string;
  readonly id: string;
  readonly contextWindowTokens: number;
  count(text: string): number;
}

export interface TokenCalibrationCase {
  readonly id: string;
  readonly category: "english" | "chinese" | "code" | "emoji" | "tool_schema";
  readonly text: string;
  readonly referenceTokens: number;
}

const fallbackProfile = {
  contextWindowTokens: 8_192,
  utf8BytesPerToken: 2,
  safetyMarginBps: 2_500
} as const;

export function parseRouteContextProfiles(raw: string): readonly RouteContextProfile[] {
  if (!raw.trim()) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    throw new Error("Model route context profiles must be valid JSON", { cause: error });
  }
  return routeContextProfilesSchema.parse(parsed);
}

export function createConservativeRouteEstimator(
  rawRoutes: readonly string[],
  rawProfiles: readonly RouteContextProfile[],
  rawTokenizers: readonly RouteTokenizerAdapter[] = []
): RouteTokenEstimator {
  const routes = rawRoutes.map((route) => route.trim());
  if (routes.length === 0 || routes.some((route) => !route) || new Set(routes).size !== routes.length) {
    throw new Error("Token estimation requires unique non-empty model routes");
  }
  const profiles = routeContextProfilesSchema.parse(rawProfiles);
  const tokenizerRoutes = rawTokenizers.map((tokenizer) => tokenizer.route.trim());
  if (rawTokenizers.some((tokenizer, index) => !tokenizer.route.trim() || !tokenizer.id.trim() || tokenizerRoutes[index] !== tokenizer.route.trim())) {
    throw new Error("Route tokenizer adapters require non-empty route and ID");
  }
  if (new Set(tokenizerRoutes).size !== tokenizerRoutes.length) {
    throw new Error("Route tokenizer adapters must be unique");
  }
  const routeSet = new Set(routes);
  for (const profile of profiles) {
    if (!routeSet.has(profile.route)) {
      throw new Error(`Model context profile references unknown route ${profile.route}`);
    }
  }
  for (const tokenizer of rawTokenizers) {
    if (!routeSet.has(tokenizer.route)) {
      throw new Error(`Route tokenizer references unknown route ${tokenizer.route}`);
    }
    if (!Number.isSafeInteger(tokenizer.contextWindowTokens) || tokenizer.contextWindowTokens < 1_024) {
      throw new Error(`Route tokenizer ${tokenizer.route} has an invalid context window`);
    }
  }
  const byRoute = new Map(profiles.map((profile) => [profile.route, profile]));
  const tokenizerByRoute = new Map(rawTokenizers.map((tokenizer) => [tokenizer.route, tokenizer]));
  const fallbackRoutes: string[] = [];
  const effective = routes.map((route) => {
    const declared = byRoute.get(route);
    if (declared !== undefined) return declared;
    fallbackRoutes.push(route);
    return { route, ...fallbackProfile };
  });
  const digest = createHash("sha256").update(JSON.stringify({
    profiles: effective,
    tokenizers: rawTokenizers.map((tokenizer) => ({ route: tokenizer.route, id: tokenizer.id, contextWindowTokens: tokenizer.contextWindowTokens }))
  }), "utf8").digest("hex");
  return {
    id: `route-calibrated-v1:sha256:${digest}`,
    contextWindowTokens: Math.min(...effective.map((profile) => profile.contextWindowTokens)),
    fallbackRoutes,
    estimate: (text: string): number => Math.max(...effective.map((profile) => {
      const tokenizer = tokenizerByRoute.get(profile.route);
      return tokenizer === undefined ? estimateForProfile(text, profile) : validTokenizerCount(tokenizer.count(text), tokenizer.route);
    }))
  };
}

export function evaluateTokenCalibration(estimator: RouteTokenEstimator, corpus: readonly TokenCalibrationCase[]): {
  readonly estimatorId: string;
  readonly cases: number;
  readonly categories: readonly TokenCalibrationCase["category"][];
  readonly underestimates: readonly { readonly id: string; readonly estimatedTokens: number; readonly referenceTokens: number }[];
} {
  const ids = new Set<string>();
  const categories = new Set<TokenCalibrationCase["category"]>();
  const underestimates: Array<{ id: string; estimatedTokens: number; referenceTokens: number }> = [];
  for (const item of corpus) {
    if (!item.id.trim() || ids.has(item.id) || !Number.isSafeInteger(item.referenceTokens) || item.referenceTokens < 1) {
      throw new Error("Token calibration cases require unique IDs and positive reference counts");
    }
    ids.add(item.id);
    categories.add(item.category);
    const estimatedTokens = estimator.estimate(item.text);
    if (estimatedTokens < item.referenceTokens) {
      underestimates.push({ id: item.id, estimatedTokens, referenceTokens: item.referenceTokens });
    }
  }
  return {
    estimatorId: estimator.id,
    cases: corpus.length,
    categories: [...categories].sort((left, right) => left.localeCompare(right)),
    underestimates
  };
}

function estimateForProfile(text: string, profile: RouteContextProfile): number {
  const raw = Buffer.byteLength(text, "utf8") / profile.utf8BytesPerToken;
  return Math.ceil(raw * (10_000 + profile.safetyMarginBps) / 10_000);
}

function validTokenizerCount(value: number, route: string): number {
  if (!Number.isSafeInteger(value) || value < 0) throw new Error(`Route tokenizer ${route} returned an invalid token count`);
  return value;
}
