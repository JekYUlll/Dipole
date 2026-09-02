import { z } from "zod";

import { parseOfflineEvalReport, type OfflineEvalReport } from "./offline-evaluator.js";

export const shadowEvalReportSchemaVersion = "dipole.agent.shadow-eval-report.v1" as const;

const traceIdSchema = z.string().trim().min(1).max(128).regex(/^[A-Za-z0-9._:-]+$/);
const reportEnvelopeSchema = z.object({
  schemaVersion: z.literal(shadowEvalReportSchemaVersion),
  traceId: traceIdSchema,
  evaluation: z.unknown()
}).strict();

export interface ShadowEvalReport {
  readonly schemaVersion: typeof shadowEvalReportSchemaVersion;
  readonly traceId: string;
  readonly evaluation: OfflineEvalReport;
}

export function createShadowEvalReport(traceId: string, evaluation: OfflineEvalReport): ShadowEvalReport {
  return parseShadowEvalReport({ schemaVersion: shadowEvalReportSchemaVersion, traceId, evaluation });
}

export function parseShadowEvalReport(value: unknown): ShadowEvalReport {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  const envelope = reportEnvelopeSchema.parse(decoded);
  return { schemaVersion: envelope.schemaVersion, traceId: envelope.traceId, evaluation: parseOfflineEvalReport(envelope.evaluation) };
}
