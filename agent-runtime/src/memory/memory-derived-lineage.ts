import { createHash } from "node:crypto";

import type { Pool, RowDataPacket } from "mysql2/promise";
import { z } from "zod";

import { GET_AGENT_MEMORY_DERIVED_IMPACT } from "./mysql-memory-derived-lineage-queries.js";

const idSchema = z.string().min(1).max(64).regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]*$/);
const manifestSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-derived-lineage-manifest.v1"),
  tenantId: idSchema,
  principalId: idSchema,
  memoryId: idSchema
}).strict();
const sha256Schema = z.string().regex(/^[a-f0-9]{64}$/);
const safeCountSchema = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const positiveSafeCountSchema = safeCountSchema.min(1);
const reportSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-derived-lineage-report.v1"),
  policyVersion: z.literal("memory-derived-lineage-v1"),
  memoryRootSha256: sha256Schema,
  lineageVersions: positiveSafeCountSchema,
  directTaskReferences: safeCountSchema,
  unindexedContextPlans: safeCountSchema,
  unattributedModelTasks: safeCountSchema,
  lineageComplete: z.boolean(),
  domains: z.object({
    modelCalls: safeCountSchema,
    shadowPlans: safeCountSchema,
    shadowSteps: safeCountSchema,
    artifacts: safeCountSchema,
    toolInvocations: safeCountSchema,
    messageActions: safeCountSchema,
    temporalHistoryPotentialTasks: safeCountSchema
  }).strict(),
  contentRead: z.literal(false),
  deletionAuthority: z.literal(false),
  runtimeAuthority: z.literal(false),
  reportSha256: sha256Schema
}).strict().superRefine((report, context) => {
  if (report.lineageComplete !== (report.unindexedContextPlans === 0 && report.unattributedModelTasks === 0)) {
    context.addIssue({ code: "custom", message: "lineage completeness is inconsistent" });
  }
  if (report.domains.temporalHistoryPotentialTasks !== report.directTaskReferences + report.unindexedContextPlans) {
    context.addIssue({ code: "custom", message: "Temporal task impact is inconsistent" });
  }
});

export interface MemoryDerivedImpactEvidence {
  memory_root_uuid: string;
  lineage_versions: number;
  direct_task_references: number;
  unindexed_context_plans: number;
  unattributed_model_tasks: number;
  model_calls: number;
  shadow_plans: number;
  shadow_steps: number;
  artifacts: number;
  tool_invocations: number;
  message_actions: number;
}

interface ImpactRow extends RowDataPacket, MemoryDerivedImpactEvidence {}

export type MemoryDerivedLineageManifest = z.infer<typeof manifestSchema>;

export type MemoryDerivedLineageReport = z.infer<typeof reportSchema>;

export interface MemoryDerivedLineageStore {
  load(manifest: MemoryDerivedLineageManifest): Promise<MemoryDerivedLineageReport>;
}

export class MySQLMemoryDerivedLineageStore implements MemoryDerivedLineageStore {
  constructor(private readonly pool: Pool) {}

  async load(rawManifest: MemoryDerivedLineageManifest): Promise<MemoryDerivedLineageReport> {
    const manifest = manifestSchema.parse(rawManifest);
    const [rows] = await this.pool.execute<ImpactRow[]>(GET_AGENT_MEMORY_DERIVED_IMPACT, [manifest.tenantId, manifest.principalId, manifest.memoryId]);
    const row = rows[0];
    if (row === undefined) throw new Error("Agent Memory derived-lineage source is unavailable");
    return buildMemoryDerivedLineageReport(row);
  }
}

export function parseMemoryDerivedLineageManifest(value: unknown): MemoryDerivedLineageManifest {
  return manifestSchema.parse(typeof value === "string" ? JSON.parse(value) as unknown : value);
}

export function parseMemoryDerivedLineageReport(value: unknown): MemoryDerivedLineageReport {
  const report = reportSchema.parse(typeof value === "string" ? JSON.parse(value) as unknown : value);
  const { reportSha256, ...core } = report;
  const expected = createHash("sha256").update(JSON.stringify(core), "utf8").digest("hex");
  if (reportSha256 !== expected) throw new Error("Agent Memory derived-lineage report hash is invalid");
  return report;
}

export function buildMemoryDerivedLineageReport(row: MemoryDerivedImpactEvidence): MemoryDerivedLineageReport {
  const counts = {
    lineageVersions: count(row.lineage_versions), directTaskReferences: count(row.direct_task_references),
    unindexedContextPlans: count(row.unindexed_context_plans), unattributedModelTasks: count(row.unattributed_model_tasks),
    modelCalls: count(row.model_calls),
    shadowPlans: count(row.shadow_plans), shadowSteps: count(row.shadow_steps), artifacts: count(row.artifacts),
    toolInvocations: count(row.tool_invocations), messageActions: count(row.message_actions)
  };
  const core = {
    schemaVersion: "dipole.agent.memory-derived-lineage-report.v1" as const,
    policyVersion: "memory-derived-lineage-v1" as const,
    memoryRootSha256: createHash("sha256").update(requiredRoot(row.memory_root_uuid), "utf8").digest("hex"),
    lineageVersions: counts.lineageVersions,
    directTaskReferences: counts.directTaskReferences,
    unindexedContextPlans: counts.unindexedContextPlans,
    unattributedModelTasks: counts.unattributedModelTasks,
    lineageComplete: counts.unindexedContextPlans === 0 && counts.unattributedModelTasks === 0,
    domains: {
      modelCalls: counts.modelCalls, shadowPlans: counts.shadowPlans, shadowSteps: counts.shadowSteps,
      artifacts: counts.artifacts, toolInvocations: counts.toolInvocations, messageActions: counts.messageActions,
      temporalHistoryPotentialTasks: sumCount(counts.directTaskReferences, counts.unindexedContextPlans)
    },
    contentRead: false as const, deletionAuthority: false as const, runtimeAuthority: false as const
  };
  return parseMemoryDerivedLineageReport({
    ...core,
    reportSha256: createHash("sha256").update(JSON.stringify(core), "utf8").digest("hex")
  });
}

function count(value: number): number {
  if (!Number.isSafeInteger(value) || value < 0) throw new Error("Agent Memory derived-lineage count is invalid");
  return value;
}

function sumCount(left: number, right: number): number {
  return count(left + right);
}

function requiredRoot(value: string): string {
  return idSchema.parse(value);
}
