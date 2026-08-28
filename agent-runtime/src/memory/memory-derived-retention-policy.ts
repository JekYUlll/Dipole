import { createHash } from "node:crypto";

import { z } from "zod";

import {
  parseMemoryDerivedLineageReport,
  type MemoryDerivedLineageReport
} from "./memory-derived-lineage.js";

const sha256Schema = z.string().regex(/^[a-f0-9]{64}$/);
const safeCountSchema = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const terminalActionSchema = z.union([
  z.object({ action: z.literal("erase_derived_content") }).strict(),
  z.object({ action: z.literal("retain_minimal_audit") }).strict(),
  z.object({ action: z.literal("expire_after_days"), retentionDays: z.number().int().min(1).max(3650) }).strict(),
  z.object({ action: z.literal("manual_review") }).strict()
]);
const domainPolicySchema = z.object({
  modelCalls: terminalActionSchema,
  shadowPlans: terminalActionSchema,
  shadowSteps: terminalActionSchema,
  artifacts: terminalActionSchema,
  toolInvocations: terminalActionSchema,
  messageActions: terminalActionSchema,
  temporalHistoryPotentialTasks: terminalActionSchema
}).strict();
const policySchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-derived-retention-policy.v1"),
  policyVersion: z.literal("memory-derived-retention-v1"),
  domains: domainPolicySchema,
  contentRead: z.literal(false),
  deletionAuthority: z.literal(false),
  runtimeAuthority: z.literal(false)
}).strict();
const blockedReasonSchema = z.enum(["lineage_incomplete", "manual_review_required"]);
const decisionDomainSchema = z.union([
  z.object({ count: safeCountSchema, action: z.literal("erase_derived_content") }).strict(),
  z.object({ count: safeCountSchema, action: z.literal("retain_minimal_audit") }).strict(),
  z.object({
    count: safeCountSchema,
    action: z.literal("expire_after_days"),
    retentionDays: z.number().int().min(1).max(3650)
  }).strict(),
  z.object({ count: safeCountSchema, action: z.literal("manual_review") }).strict()
]);
const decisionSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-derived-retention-decision.v1"),
  policyVersion: z.literal("memory-derived-retention-v1"),
  memoryRootSha256: sha256Schema,
  lineageReportSha256: sha256Schema,
  policySha256: sha256Schema,
  lineageComplete: z.boolean(),
  policyComplete: z.boolean(),
  blockedReasons: z.array(blockedReasonSchema).max(2),
  domains: z.object({
    modelCalls: decisionDomainSchema,
    shadowPlans: decisionDomainSchema,
    shadowSteps: decisionDomainSchema,
    artifacts: decisionDomainSchema,
    toolInvocations: decisionDomainSchema,
    messageActions: decisionDomainSchema,
    temporalHistoryPotentialTasks: decisionDomainSchema
  }).strict(),
  contentRead: z.literal(false),
  deletionExecuted: z.literal(false),
  deletionAuthority: z.literal(false),
  runtimeAuthority: z.literal(false),
  decisionSha256: sha256Schema
}).strict().superRefine((decision, context) => {
  if (new Set(decision.blockedReasons).size !== decision.blockedReasons.length) {
    context.addIssue({ code: "custom", message: "blocked reasons must be unique" });
  }
  const expectedReasons: Array<z.infer<typeof blockedReasonSchema>> = [];
  if (!decision.lineageComplete) expectedReasons.push("lineage_incomplete");
  if (Object.values(decision.domains).some(domain => domain.count > 0 && domain.action === "manual_review")) {
    expectedReasons.push("manual_review_required");
  }
  if (decision.blockedReasons.join("|") !== expectedReasons.join("|")) {
    context.addIssue({ code: "custom", message: "blocked reasons are inconsistent" });
  }
  if (decision.policyComplete !== (expectedReasons.length === 0)) {
    context.addIssue({ code: "custom", message: "policy completeness is inconsistent" });
  }
});

export type MemoryDerivedRetentionPolicy = z.infer<typeof policySchema>;
export type MemoryDerivedRetentionDecision = z.infer<typeof decisionSchema>;

export function parseMemoryDerivedRetentionPolicy(value: unknown): MemoryDerivedRetentionPolicy {
  return policySchema.parse(parseJSON(value));
}

export function parseMemoryDerivedRetentionDecision(value: unknown): MemoryDerivedRetentionDecision {
  const decision = decisionSchema.parse(parseJSON(value));
  const { decisionSha256, ...core } = decision;
  if (decisionSha256 !== sha256(core)) throw new Error("Agent Memory derived retention decision hash is invalid");
  return decision;
}

export function buildMemoryDerivedRetentionDecision(
  rawPolicy: MemoryDerivedRetentionPolicy,
  rawReport: MemoryDerivedLineageReport
): MemoryDerivedRetentionDecision {
  const policy = parseMemoryDerivedRetentionPolicy(rawPolicy);
  const report = parseMemoryDerivedLineageReport(rawReport);
  const blockedReasons: Array<z.infer<typeof blockedReasonSchema>> = [];
  if (!report.lineageComplete) blockedReasons.push("lineage_incomplete");

  const domains = mapDomains(policy, report);
  if (Object.values(domains).some(domain => domain.count > 0 && domain.action === "manual_review")) {
    blockedReasons.push("manual_review_required");
  }
  const core = {
    schemaVersion: "dipole.agent.memory-derived-retention-decision.v1" as const,
    policyVersion: "memory-derived-retention-v1" as const,
    memoryRootSha256: report.memoryRootSha256,
    lineageReportSha256: report.reportSha256,
    policySha256: sha256(policy),
    lineageComplete: report.lineageComplete,
    policyComplete: blockedReasons.length === 0,
    blockedReasons,
    domains,
    contentRead: false as const,
    deletionExecuted: false as const,
    deletionAuthority: false as const,
    runtimeAuthority: false as const
  };
  return parseMemoryDerivedRetentionDecision({ ...core, decisionSha256: sha256(core) });
}

function mapDomains(policy: MemoryDerivedRetentionPolicy, report: MemoryDerivedLineageReport) {
  return {
    modelCalls: { count: report.domains.modelCalls, ...policy.domains.modelCalls },
    shadowPlans: { count: report.domains.shadowPlans, ...policy.domains.shadowPlans },
    shadowSteps: { count: report.domains.shadowSteps, ...policy.domains.shadowSteps },
    artifacts: { count: report.domains.artifacts, ...policy.domains.artifacts },
    toolInvocations: { count: report.domains.toolInvocations, ...policy.domains.toolInvocations },
    messageActions: { count: report.domains.messageActions, ...policy.domains.messageActions },
    temporalHistoryPotentialTasks: {
      count: report.domains.temporalHistoryPotentialTasks,
      ...policy.domains.temporalHistoryPotentialTasks
    }
  };
}

function parseJSON(value: unknown): unknown {
  return typeof value === "string" ? JSON.parse(value) as unknown : value;
}

function sha256(value: unknown): string {
  return createHash("sha256").update(JSON.stringify(value), "utf8").digest("hex");
}
