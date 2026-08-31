import { z } from "zod";

import { offlineEvalCategories, parseOfflineEvalReport, type OfflineEvalReport } from "./offline-evaluator.js";

export const shadowEvalSummaryInputSchemaVersion = "dipole.agent.shadow-eval-summary-input.v1" as const;
export const shadowEvalSummaryReportSchemaVersion = "dipole.agent.shadow-eval-summary-report.v1" as const;

const sha256Schema = z.string().regex(/^[a-f0-9]{64}$/);
const sourceSchema = z.object({
  kind: z.literal("reviewed_shadow"),
  environment: z.enum(["isolated", "shared_development"]),
  runtimeRevision: sha256Schema,
  windowStart: z.string().datetime({ offset: true }),
  windowEnd: z.string().datetime({ offset: true })
}).strict();
const summaryInputSchema = z.object({
  schemaVersion: z.literal(shadowEvalSummaryInputSchemaVersion),
  source: sourceSchema,
  reports: z.array(z.unknown()).min(1).max(1_000)
}).strict();

export interface ShadowEvalSummaryInput {
  readonly schemaVersion: typeof shadowEvalSummaryInputSchemaVersion;
  readonly source: z.infer<typeof sourceSchema>;
  readonly reports: readonly OfflineEvalReport[];
}

export interface ShadowEvalSummaryReport {
  readonly schemaVersion: typeof shadowEvalSummaryReportSchemaVersion;
  readonly source: z.infer<typeof sourceSchema>;
  readonly candidateVersion: string;
  readonly summary: {
    readonly evaluatedTasks: number;
    readonly succeededTasks: number;
    readonly failedTasks: number;
    readonly taskSuccessRatePercent: number;
    readonly categoryPassRates: Record<(typeof offlineEvalCategories)[number], {
      readonly total: number;
      readonly passed: number;
      readonly passRatePercent: number;
    }>;
    readonly failureReasons: readonly {
      readonly category: (typeof offlineEvalCategories)[number];
      readonly reason: string;
      readonly count: number;
    }[];
  };
  readonly evaluationSuiteSha256: readonly string[];
  readonly limitations: readonly string[];
}

export function parseShadowEvalSummaryInput(value: unknown): ShadowEvalSummaryInput {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  const input = summaryInputSchema.parse(decoded);
  if (Date.parse(input.source.windowStart) > Date.parse(input.source.windowEnd)) {
    throw new Error("Shadow evaluation summary window start must not be after window end");
  }

  const reports = input.reports.map(parseOfflineEvalReport);
  for (const report of reports) assertShadowTaskReport(report);
  const candidateVersions = new Set(reports.map(report => report.candidateVersion));
  if (candidateVersions.size !== 1) throw new Error("Shadow evaluation summary reports must share one candidate version");
  const suiteHashes = reports.map(report => report.suiteSha256);
  if (new Set(suiteHashes).size !== suiteHashes.length) {
    throw new Error("Shadow evaluation summary reports must have unique suite SHA-256 values");
  }

  return { schemaVersion: input.schemaVersion, source: input.source, reports };
}

function assertShadowTaskReport(report: OfflineEvalReport): void {
  for (const category of offlineEvalCategories) {
    const matching = report.cases.filter(item => item.category === category);
    if (matching.length !== 1 || !new RegExp(`^${category}\\.shadow\\.[a-f0-9]{24}$`).test(matching[0]!.id)) {
      throw new Error("Shadow evaluation summary accepts exactly one bound Shadow case per evaluation category");
    }
  }
}

export function summarizeShadowEvalReports(input: ShadowEvalSummaryInput): ShadowEvalSummaryReport {
  const validated = parseShadowEvalSummaryInput(input);
  const reports = validated.reports;
  const succeededTasks = reports.filter(report => report.passed).length;
  const categoryPassRates = Object.fromEntries(offlineEvalCategories.map(category => {
    const cases = reports.flatMap(report => report.cases.filter(item => item.category === category));
    const passed = cases.filter(item => item.passed).length;
    return [category, { total: cases.length, passed, passRatePercent: percentage(passed, cases.length) }];
  })) as ShadowEvalSummaryReport["summary"]["categoryPassRates"];
  const failureReasons = [...failureReasonCounts(reports).entries()]
    .map(([key, count]) => {
      const [category, reason] = key.split("\u0000", 2) as [(typeof offlineEvalCategories)[number], string];
      return { category, reason, count };
    })
    .sort((left, right) => right.count - left.count || left.category.localeCompare(right.category) || left.reason.localeCompare(right.reason));

  return {
    schemaVersion: shadowEvalSummaryReportSchemaVersion,
    source: validated.source,
    candidateVersion: reports[0]!.candidateVersion,
    summary: {
      evaluatedTasks: reports.length,
      succeededTasks,
      failedTasks: reports.length - succeededTasks,
      taskSuccessRatePercent: percentage(succeededTasks, reports.length),
      categoryPassRates,
      failureReasons
    },
    evaluationSuiteSha256: reports.map(report => report.suiteSha256).sort(),
    limitations: [
      "Only reviewed terminal Shadow Task/Run reports are included.",
      "This report does not establish production authority, active-runtime quality, or user impact.",
      "Task, message, prompt, model output, tool arguments, artifact body, and principal identifiers are excluded."
    ]
  };
}

function failureReasonCounts(reports: readonly OfflineEvalReport[]): Map<string, number> {
  const counts = new Map<string, number>();
  for (const report of reports) {
    for (const item of report.cases) {
      for (const reason of item.reasons) {
        const key = `${item.category}\u0000${reason}`;
        counts.set(key, (counts.get(key) ?? 0) + 1);
      }
    }
  }
  return counts;
}

function percentage(numerator: number, denominator: number): number {
  return Math.round((numerator / denominator) * 10_000) / 100;
}
