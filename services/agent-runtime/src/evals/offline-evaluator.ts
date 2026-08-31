import { createHash } from "node:crypto";

import { z } from "zod";

export const offlineEvalCategories = ["outcome", "trajectory", "permission", "retrieval", "cost"] as const;
export type OfflineEvalCategory = (typeof offlineEvalCategories)[number];

const identifierSchema = z.string().trim().min(2).max(128).regex(/^[a-z0-9][a-z0-9._:-]*$/);
const resourceIdSchema = z.union([z.literal("*"), identifierSchema]);
const candidateVersionSchema = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/);
const identifierSequenceSchema = z.array(identifierSchema).max(256);
const identifierListSchema = identifierSequenceSchema.refine(values => new Set(values).size === values.length, "identifiers must be unique");
const baseCaseSchema = z.object({ id: identifierSchema }).strict();

const outcomeCaseSchema = baseCaseSchema.extend({
  category: z.literal("outcome"),
  expected: z.object({
    requiredOutputIds: identifierListSchema.min(1),
    forbiddenOutputIds: identifierListSchema
  }).strict(),
  observed: z.object({ outputIds: identifierListSchema }).strict()
}).strict();

const trajectoryCaseSchema = baseCaseSchema.extend({
  category: z.literal("trajectory"),
  expected: z.object({
    steps: identifierSequenceSchema,
    forbiddenSteps: identifierListSchema
  }).strict(),
  observed: z.object({ steps: identifierSequenceSchema }).strict()
}).strict();

const permissionDecisionSchema = z.object({
  capabilityId: identifierSchema,
  resourceType: identifierSchema,
  resourceId: resourceIdSchema,
  action: identifierSchema,
  decision: z.enum(["allowed", "denied"])
}).strict();

const permissionDecisionsSchema = z.array(permissionDecisionSchema).max(256).superRefine((decisions, context) => {
  const bindings = decisions.map(permissionBinding);
  if (new Set(bindings).size !== bindings.length) context.addIssue({ code: "custom", message: "permission bindings must be unique" });
});

const permissionCaseSchema = baseCaseSchema.extend({
  category: z.literal("permission"),
  expected: z.object({ decisions: permissionDecisionsSchema }).strict(),
  observed: z.object({ decisions: permissionDecisionsSchema }).strict()
}).strict();

const retrievalCaseSchema = baseCaseSchema.extend({
  category: z.literal("retrieval"),
  expected: z.object({
    relevantEvidenceIds: identifierListSchema.min(1),
    minimumRecall: z.number().min(0).max(1),
    minimumPrecision: z.number().min(0).max(1)
  }).strict(),
  observed: z.object({ retrievedEvidenceIds: identifierListSchema }).strict()
}).strict();

const costMetricsSchema = z.object({
  modelCalls: z.number().int().nonnegative(),
  toolCalls: z.number().int().nonnegative(),
  totalTokens: z.number().int().nonnegative(),
  totalCostMicrousd: z.number().int().nonnegative(),
  latencyMs: z.number().int().nonnegative()
}).strict();

const costCaseSchema = baseCaseSchema.extend({
  category: z.literal("cost"),
  expected: z.object({ maximums: costMetricsSchema }).strict(),
  observed: costMetricsSchema
}).strict();

const offlineEvalCaseSchema = z.discriminatedUnion("category", [
  outcomeCaseSchema, trajectoryCaseSchema, permissionCaseSchema, retrievalCaseSchema, costCaseSchema
]);

const offlineEvalSuiteSchema = z.object({
  schemaVersion: z.literal("dipole.agent.offline-eval-suite.v1"),
  candidateVersion: candidateVersionSchema,
  cases: z.array(offlineEvalCaseSchema).min(5).max(1000)
}).strict().superRefine((suite, context) => {
  const ids = suite.cases.map(item => item.id);
  if (new Set(ids).size !== ids.length) context.addIssue({ code: "custom", message: "case ids must be unique" });
  const categories = new Set(suite.cases.map(item => item.category));
  if (offlineEvalCategories.some(category => !categories.has(category))) {
    context.addIssue({ code: "custom", message: "suite must contain all five evaluation categories" });
  }
});

export type OfflineEvalSuite = z.infer<typeof offlineEvalSuiteSchema>;
type OfflineEvalCase = OfflineEvalSuite["cases"][number];

export interface OfflineEvalCaseResult {
  id: string;
  category: OfflineEvalCategory;
  passed: boolean;
  reasons: string[];
  metrics?: Record<string, number>;
}

export interface OfflineEvalReport {
  schemaVersion: "dipole.agent.offline-eval-report.v1";
  candidateVersion: string;
  suiteSha256: string;
  passed: boolean;
  summary: {
    total: number;
    passed: number;
    categories: Record<OfflineEvalCategory, { total: number; passed: number }>;
  };
  cases: OfflineEvalCaseResult[];
}

const categorySummarySchema = z.object({ total: z.number().int().positive(), passed: z.number().int().nonnegative() }).strict();
const offlineEvalReportSchema = z.object({
  schemaVersion: z.literal("dipole.agent.offline-eval-report.v1"),
  candidateVersion: candidateVersionSchema,
  suiteSha256: z.string().regex(/^[a-f0-9]{64}$/),
  passed: z.boolean(),
  summary: z.object({
    total: z.number().int().positive(),
    passed: z.number().int().nonnegative(),
    categories: z.record(z.enum(offlineEvalCategories), categorySummarySchema)
  }).strict(),
  cases: z.array(z.object({
    id: identifierSchema,
    category: z.enum(offlineEvalCategories),
    passed: z.boolean(),
    reasons: identifierListSchema,
    metrics: z.record(z.string().min(1).max(64), z.number().nonnegative()).optional()
  }).strict()).min(5).max(1000)
}).strict().superRefine((report, context) => {
  if (report.cases.some(item => item.passed !== (item.reasons.length === 0))) {
    context.addIssue({ code: "custom", message: "offline evaluation case result conflicts with reasons" });
  }
  const passed = report.cases.filter(item => item.passed).length;
  if (report.summary.total !== report.cases.length || report.summary.passed !== passed || report.passed !== (passed === report.cases.length)) {
    context.addIssue({ code: "custom", message: "offline evaluation report summary is inconsistent" });
  }
  for (const category of offlineEvalCategories) {
    const cases = report.cases.filter(item => item.category === category);
    const summary = report.summary.categories[category];
    if (summary.total !== cases.length || summary.passed !== cases.filter(item => item.passed).length) {
      context.addIssue({ code: "custom", message: `offline evaluation ${category} summary is inconsistent` });
    }
  }
});

export function parseOfflineEvalSuite(value: unknown): OfflineEvalSuite {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  return offlineEvalSuiteSchema.parse(decoded);
}

export function parseOfflineEvalReport(value: unknown): OfflineEvalReport {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  return offlineEvalReportSchema.parse(decoded) as OfflineEvalReport;
}

export function evaluateOfflineEvalSuite(suite: OfflineEvalSuite): OfflineEvalReport {
  const validated = offlineEvalSuiteSchema.parse(suite);
  const cases = validated.cases.map(evaluateCase);
  const categories = Object.fromEntries(offlineEvalCategories.map(category => {
    const matching = cases.filter(item => item.category === category);
    return [category, { total: matching.length, passed: matching.filter(item => item.passed).length }];
  })) as OfflineEvalReport["summary"]["categories"];

  return {
    schemaVersion: "dipole.agent.offline-eval-report.v1",
    candidateVersion: validated.candidateVersion,
    suiteSha256: createHash("sha256").update(canonicalJSON(validated)).digest("hex"),
    passed: cases.every(item => item.passed),
    summary: { total: cases.length, passed: cases.filter(item => item.passed).length, categories },
    cases
  };
}

function evaluateCase(testCase: OfflineEvalCase): OfflineEvalCaseResult {
  const reasons: string[] = [];
  let metrics: Record<string, number> | undefined;

  switch (testCase.category) {
    case "outcome": {
      const outputs = new Set(testCase.observed.outputIds);
      if (testCase.expected.forbiddenOutputIds.some(id => outputs.has(id))) reasons.push("forbidden_output");
      if (testCase.expected.requiredOutputIds.some(id => !outputs.has(id))) reasons.push("missing_required_output");
      break;
    }
    case "trajectory":
      if (testCase.expected.forbiddenSteps.some(step => testCase.observed.steps.includes(step))) reasons.push("forbidden_step");
      if (!equalList(testCase.expected.steps, testCase.observed.steps)) reasons.push("trajectory_mismatch");
      break;
    case "permission": {
      const expected = testCase.expected.decisions.map(decisionValue).sort();
      const observed = testCase.observed.decisions.map(decisionValue).sort();
      if (!equalList(expected, observed)) reasons.push("permission_decision_mismatch");
      break;
    }
    case "retrieval": {
      const relevant = new Set(testCase.expected.relevantEvidenceIds);
      const hits = testCase.observed.retrievedEvidenceIds.filter(id => relevant.has(id)).length;
      const recall = hits / relevant.size;
      const precision = testCase.observed.retrievedEvidenceIds.length === 0 ? 0 : hits / testCase.observed.retrievedEvidenceIds.length;
      metrics = { precision, recall };
      if (precision < testCase.expected.minimumPrecision) reasons.push("retrieval_precision_below_minimum");
      if (recall < testCase.expected.minimumRecall) reasons.push("retrieval_recall_below_minimum");
      break;
    }
    case "cost": {
      metrics = { ...testCase.observed };
      for (const [metric, maximum] of Object.entries(testCase.expected.maximums)) {
        if (testCase.observed[metric as keyof typeof testCase.observed] > maximum) reasons.push(`${camelToSnake(metric)}_exceeded`);
      }
      break;
    }
  }

  reasons.sort();
  const result: OfflineEvalCaseResult = { id: testCase.id, category: testCase.category, passed: reasons.length === 0, reasons };
  return metrics === undefined ? result : { ...result, metrics };
}

function permissionBinding(decision: z.infer<typeof permissionDecisionSchema>): string {
  return [decision.capabilityId, decision.resourceType, decision.resourceId, decision.action].join("\u0000");
}

function decisionValue(decision: z.infer<typeof permissionDecisionSchema>): string {
  return `${permissionBinding(decision)}\u0000${decision.decision}`;
}

function equalList(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function camelToSnake(value: string): string {
  return value.replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`);
}

export function canonicalJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(",")}]`;
  if (value !== null && typeof value === "object") {
    return `{${Object.entries(value).sort(([left], [right]) => left < right ? -1 : left > right ? 1 : 0).map(([key, item]) => `${JSON.stringify(key)}:${canonicalJSON(item)}`).join(",")}}`;
  }
  return JSON.stringify(value);
}
