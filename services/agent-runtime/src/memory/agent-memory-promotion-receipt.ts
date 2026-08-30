import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";
import {
  agentMemoryTypeSchema,
  validateMemoryTypeTransition,
  type AgentMemoryType
} from "./memory-type-policy.js";

const schemaVersion = "dipole.agent.memory-promotion-receipt.v1" as const;
const maxReceiptLifetimeMs = 15 * 60 * 1000;
const identity = z.string().trim().min(1).max(128);
const hash = z.string().regex(/^[a-f0-9]{64}$/);
const timestamp = z.string().datetime({ offset: true });

const intentSchema = z.object({
  tenantId: identity, principalUserId: identity, agentId: identity, taskId: identity, runId: identity,
  candidateId: identity, candidateSha256: hash, reviewId: identity, policyVersion: identity,
  candidateMemoryType: agentMemoryTypeSchema, targetMemoryType: agentMemoryTypeSchema,
  expiresAt: timestamp
}).strict();

const receiptSchema = z.object({
  schemaVersion: z.literal(schemaVersion), receiptId: z.string().regex(/^MEM-PROMOTE-[a-f0-9]{64}$/),
  receiptSha256: hash, status: z.enum(["prepared", "committed", "cancelled", "expired"]),
  tenantId: identity, principalUserId: identity, agentId: identity, taskId: identity, runId: identity,
  candidateId: identity, candidateSha256: hash, reviewId: identity, policyVersion: identity,
  candidateMemoryType: agentMemoryTypeSchema, targetMemoryType: agentMemoryTypeSchema,
  createdAt: timestamp, expiresAt: timestamp
}).strict();

export interface AgentMemoryPromotionIntent {
  readonly tenantId: string;
  readonly principalUserId: string;
  readonly agentId: string;
  readonly taskId: string;
  readonly runId: string;
  readonly candidateId: string;
  readonly candidateSha256: string;
  readonly reviewId: string;
  readonly policyVersion: string;
  readonly candidateMemoryType: AgentMemoryType;
  readonly targetMemoryType: AgentMemoryType;
  readonly expiresAt: string;
}

export interface AgentMemoryPromotionReceipt extends AgentMemoryPromotionIntent {
  readonly schemaVersion: typeof schemaVersion;
  readonly receiptId: string;
  readonly receiptSha256: string;
  readonly status: "prepared" | "committed" | "cancelled" | "expired";
  readonly createdAt: string;
}

export function createAgentMemoryPromotionReceipt(
  rawIntent: AgentMemoryPromotionIntent,
  now: Date = new Date()
): AgentMemoryPromotionReceipt {
  const intent = intentSchema.parse(rawIntent);
  validateMemoryTypeTransition(intent.candidateMemoryType, intent.targetMemoryType);
  const createdAt = validNow(now);
  const expiresAt = validTimestamp(intent.expiresAt, "expiry");
  if (expiresAt <= createdAt || expiresAt - createdAt > maxReceiptLifetimeMs) {
    throw new Error("Agent Memory promotion receipt expiry is invalid");
  }
  const body = {
    schemaVersion,
    status: "prepared" as const,
    tenantId: intent.tenantId, principalUserId: intent.principalUserId, agentId: intent.agentId,
    taskId: intent.taskId, runId: intent.runId, candidateId: intent.candidateId,
    candidateSha256: intent.candidateSha256, reviewId: intent.reviewId, policyVersion: intent.policyVersion,
    candidateMemoryType: intent.candidateMemoryType, targetMemoryType: intent.targetMemoryType,
    createdAt: new Date(createdAt).toISOString(), expiresAt: new Date(expiresAt).toISOString()
  } as const;
  const receiptSha256 = sha256(body);
  return {
    ...body,
    receiptId: `MEM-PROMOTE-${sha256({ ...body, receiptSha256 })}`,
    receiptSha256,
    status: "prepared"
  };
}

export function validateAgentMemoryPromotionReceipt(raw: unknown): AgentMemoryPromotionReceipt {
  const receipt = receiptSchema.parse(raw);
  validateMemoryTypeTransition(receipt.candidateMemoryType, receipt.targetMemoryType);
  const body = {
    schemaVersion,
    status: receipt.status,
    tenantId: receipt.tenantId, principalUserId: receipt.principalUserId, agentId: receipt.agentId,
    taskId: receipt.taskId, runId: receipt.runId, candidateId: receipt.candidateId,
    candidateSha256: receipt.candidateSha256, reviewId: receipt.reviewId, policyVersion: receipt.policyVersion,
    candidateMemoryType: receipt.candidateMemoryType, targetMemoryType: receipt.targetMemoryType,
    createdAt: receipt.createdAt, expiresAt: receipt.expiresAt
  } as const;
  if (receipt.receiptSha256 !== sha256(body) || receipt.receiptId !== `MEM-PROMOTE-${sha256({ ...body, receiptSha256: receipt.receiptSha256 })}`) {
    throw new Error("Agent Memory promotion receipt hash is invalid");
  }
  const createdAt = validTimestamp(receipt.createdAt, "creation");
  const expiresAt = validTimestamp(receipt.expiresAt, "expiry");
  if (expiresAt <= createdAt || expiresAt - createdAt > maxReceiptLifetimeMs) {
    throw new Error("Agent Memory promotion receipt time window is invalid");
  }
  return receipt;
}

export function replayAgentMemoryPromotionReceipt(
  rawReceipt: unknown,
  rawIntent: AgentMemoryPromotionIntent,
  now: Date = new Date()
): AgentMemoryPromotionReceipt {
  const receipt = validateAgentMemoryPromotionReceipt(rawReceipt);
  const intent = intentSchema.parse(rawIntent);
  if (receipt.status !== "prepared") throw new Error("Agent Memory promotion receipt is no longer replayable");
  if (validNow(now) >= Date.parse(receipt.expiresAt)) throw new Error("Agent Memory promotion receipt is expired");
  if (receipt.tenantId !== intent.tenantId || receipt.principalUserId !== intent.principalUserId || receipt.agentId !== intent.agentId ||
      receipt.taskId !== intent.taskId || receipt.runId !== intent.runId || receipt.candidateId !== intent.candidateId ||
      receipt.candidateSha256 !== intent.candidateSha256 || receipt.reviewId !== intent.reviewId || receipt.policyVersion !== intent.policyVersion ||
      receipt.candidateMemoryType !== intent.candidateMemoryType || receipt.targetMemoryType !== intent.targetMemoryType ||
      receipt.expiresAt !== new Date(validTimestamp(intent.expiresAt, "expiry")).toISOString()) {
    throw new Error("Agent Memory promotion receipt intent conflict");
  }
  return receipt;
}

function validNow(value: Date): number {
  const time = value.getTime();
  if (!Number.isFinite(time)) throw new Error("Agent Memory promotion receipt clock is invalid");
  return time;
}

function validTimestamp(value: string, label: string): number {
  const time = Date.parse(value);
  if (!Number.isFinite(time) || new Date(time).toISOString() !== new Date(value).toISOString()) {
    throw new Error(`Agent Memory promotion receipt ${label} timestamp is invalid`);
  }
  return time;
}

function sha256(value: unknown): string {
  return createHash("sha256").update(canonicalMcpJSON(value), "utf8").digest("hex");
}
