import { createHash } from "node:crypto";

import { z } from "zod";

import { createConservativeRouteEstimator, routeContextProfileSchema } from "./token-estimator.js";

const categories = ["chinese", "code", "emoji", "english", "tool_schema"] as const;
const categorySchema = z.enum(categories);

const referenceSourceSchema = z.object({
  kind: z.enum(["provider_tokenizer", "provider_usage"]),
  provider: z.string().trim().min(1).max(128),
  model: z.string().trim().min(1).max(255),
  revision: z.string().trim().min(1).max(255)
}).strict();

const calibrationCaseSchema = z.object({
  id: z.string().trim().min(1).max(128),
  route: z.string().trim().min(1).max(255),
  category: categorySchema,
  text: z.string().min(1).max(100_000),
  referenceTokens: z.number().int().min(1).max(10_000_000),
  source: referenceSourceSchema
}).strict();

export const tokenCalibrationEvidenceSchema = z.object({
  version: z.literal("dipole.agent.context-calibration.evidence.v1"),
  candidate: z.string().regex(/^[a-f0-9]{40}$/),
  capturedAt: z.iso.datetime(),
  dataClassification: z.literal("synthetic"),
  routes: z.array(z.string().trim().min(1).max(255)).min(1),
  profiles: z.array(routeContextProfileSchema),
  cases: z.array(calibrationCaseSchema).min(1)
}).strict().superRefine((evidence, refinement) => {
  if (new Set(evidence.routes).size !== evidence.routes.length) {
    refinement.addIssue({ code: "custom", message: "Calibration routes must be unique", path: ["routes"] });
  }
  if (new Set(evidence.cases.map((item) => item.id)).size !== evidence.cases.length) {
    refinement.addIssue({ code: "custom", message: "Calibration case IDs must be unique", path: ["cases"] });
  }
  const routes = new Set(evidence.routes);
  for (const [index, item] of evidence.cases.entries()) {
    if (!routes.has(item.route)) {
      refinement.addIssue({ code: "custom", message: "Calibration case must reference a configured route", path: ["cases", index, "route"] });
    }
  }
  for (const route of evidence.routes) {
    const covered = new Set(evidence.cases.filter((item) => item.route === route).map((item) => item.category));
    if (categories.some((category) => !covered.has(category))) {
      refinement.addIssue({ code: "custom", message: `Calibration route ${route} must cover all required categories`, path: ["cases"] });
    }
  }
});

export type TokenCalibrationEvidence = z.infer<typeof tokenCalibrationEvidenceSchema>;

export interface TokenCalibrationMeasurement {
  readonly id: string;
  readonly route: string;
  readonly category: typeof categories[number];
  readonly textSha256: string;
  readonly utf8Bytes: number;
  readonly referenceTokens: number;
  readonly estimatedTokens: number;
  readonly errorBps: number;
  readonly source: z.infer<typeof referenceSourceSchema>;
}

export interface TokenCalibrationReport {
  readonly version: "dipole.agent.context-calibration.report.v1";
  readonly candidate: string;
  readonly evidenceCapturedAt: string;
  readonly evidenceSha256: string;
  readonly estimatorId: string;
  readonly contextWindowTokens: number;
  readonly fallbackRoutes: readonly string[];
  readonly caseCount: number;
  readonly categories: readonly typeof categories[number][];
  readonly measurements: readonly TokenCalibrationMeasurement[];
  readonly underestimates: readonly Pick<TokenCalibrationMeasurement, "id" | "route" | "estimatedTokens" | "referenceTokens">[];
  readonly eligible: boolean;
  readonly reportSha256: string;
}

export function parseCalibrationEvidenceJSON(raw: string): TokenCalibrationEvidence {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch (error) {
    throw new Error("Calibration evidence must contain a single JSON value", { cause: error });
  }
  return tokenCalibrationEvidenceSchema.parse(parsed);
}

export function evaluateCalibrationEvidence(rawEvidence: TokenCalibrationEvidence): TokenCalibrationReport {
  const evidence = tokenCalibrationEvidenceSchema.parse(rawEvidence);
  const aggregateEstimator = createConservativeRouteEstimator(evidence.routes, evidence.profiles);
  const byRoute = new Map(evidence.routes.map((route) => [
    route,
    createConservativeRouteEstimator([route], evidence.profiles.filter((profile) => profile.route === route))
  ]));
  const measurements = evidence.cases.map((item): TokenCalibrationMeasurement => {
    const estimatedTokens = byRoute.get(item.route)!.estimate(item.text);
    return {
      id: item.id,
      route: item.route,
      category: item.category,
      textSha256: sha256(item.text),
      utf8Bytes: Buffer.byteLength(item.text, "utf8"),
      referenceTokens: item.referenceTokens,
      estimatedTokens,
      errorBps: Math.round((estimatedTokens - item.referenceTokens) * 10_000 / item.referenceTokens),
      source: { ...item.source }
    };
  }).sort((left, right) => left.route.localeCompare(right.route) || left.id.localeCompare(right.id));
  const underestimates = measurements
    .filter((item) => item.estimatedTokens < item.referenceTokens)
    .map((item) => ({
      id: item.id,
      route: item.route,
      estimatedTokens: item.estimatedTokens,
      referenceTokens: item.referenceTokens
    }));
  const report = {
    version: "dipole.agent.context-calibration.report.v1" as const,
    candidate: evidence.candidate,
    evidenceCapturedAt: evidence.capturedAt,
    evidenceSha256: sha256(canonicalJSON(evidence)),
    estimatorId: aggregateEstimator.id,
    contextWindowTokens: aggregateEstimator.contextWindowTokens,
    fallbackRoutes: aggregateEstimator.fallbackRoutes,
    caseCount: measurements.length,
    categories: [...new Set(measurements.map((item) => item.category))].sort((left, right) => left.localeCompare(right)),
    measurements,
    underestimates,
    eligible: aggregateEstimator.fallbackRoutes.length === 0 && underestimates.length === 0
  };
  return { ...report, reportSha256: sha256(canonicalJSON(report)) };
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}

function canonicalJSON(value: unknown): string {
  return JSON.stringify(sortValue(value));
}

function sortValue(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(sortValue);
  if (typeof value !== "object" || value === null) return value;
  return Object.fromEntries(Object.entries(value).sort(([left], [right]) => left.localeCompare(right)).map(
    ([key, item]) => [key, sortValue(item)]
  ));
}
