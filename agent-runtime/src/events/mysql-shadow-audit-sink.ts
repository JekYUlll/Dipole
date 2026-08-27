import { createHash, randomUUID } from "node:crypto";

import type { Pool, ResultSetHeader, RowDataPacket } from "mysql2/promise";

import type { ShadowAuditRecord, ShadowAuditSink } from "./shadow-processor.js";
import {
  CLAIM_AGENT_SHADOW_STEP,
  COMPLETE_AGENT_SHADOW_STEP,
  FAIL_AGENT_SHADOW_STEP,
  GET_AGENT_SHADOW_PLAN,
  GET_AGENT_SHADOW_STEP,
  INSERT_AGENT_SHADOW_PLAN,
  INSERT_AGENT_SHADOW_STEP
} from "./mysql-shadow-audit-queries.js";

interface ExistingPlanRow extends RowDataPacket {
  task_uuid: string;
  event_id: string;
  event_type: string;
  plan_sha256: string;
}

interface ExistingStepRow extends RowDataPacket {
  status: "planned" | "running" | "completed" | "failed" | "denied";
  claim_token: string | null;
}

export type ShadowStepClaim =
  | { readonly outcome: "claimed"; readonly token: string }
  | { readonly outcome: "completed" }
  | { readonly outcome: "busy" };

export class MySQLShadowAuditSink implements ShadowAuditSink {
  constructor(private readonly pool: Pool) {}

  async append(record: ShadowAuditRecord): Promise<void> {
    const canonicalPlan = canonicalJSON(record.plan);
    const planHash = createHash("sha256").update(canonicalPlan, "utf8").digest("hex");
    const connection = await this.pool.getConnection();
    try {
      await connection.beginTransaction();
      const model = record.plan.model;
      let inserted = false;
      try {
        const [result] = await connection.execute<ResultSetHeader>(INSERT_AGENT_SHADOW_PLAN, [
          required(record.taskId, "Task ID"), required(record.eventId, "event ID"), required(record.eventType, "event type"),
          required(record.plan.summary, "plan summary"), planHash, model?.route ?? null, model?.attempts ?? null,
          model?.inputTokens ?? null, model?.outputTokens ?? null
        ]);
        inserted = result.affectedRows === 1;
      } catch (error) {
        if (!isDuplicateKey(error)) {
          throw error;
        }
      }
      if (!inserted) {
        const [rows] = await connection.execute<ExistingPlanRow[]>(GET_AGENT_SHADOW_PLAN, [record.taskId]);
        const existing = rows[0];
        if (existing === undefined || existing.event_id !== record.eventId || existing.event_type !== record.eventType || existing.plan_sha256 !== planHash) {
          throw new Error(`Agent shadow plan conflict for Task ${record.taskId}`);
        }
        await connection.commit();
        return;
      }
      for (const [index, step] of record.plan.steps.entries()) {
        await connection.execute(INSERT_AGENT_SHADOW_STEP, [
          record.taskId, index + 1, required(step.capabilityId, "capability ID"), canonicalJSON(step.input)
        ]);
      }
      await connection.commit();
    } catch (error) {
      await connection.rollback().catch(() => undefined);
      throw error;
    } finally {
      connection.release();
    }
  }

  async claimStep(taskId: string, stepNo: number, leaseMs: number): Promise<ShadowStepClaim> {
    if (!Number.isSafeInteger(stepNo) || stepNo < 1 || !Number.isSafeInteger(leaseMs) || leaseMs < 1_000) {
      throw new Error("Agent shadow Step number and lease are invalid");
    }
    const token = randomUUID();
    const [result] = await this.pool.execute<ResultSetHeader>(CLAIM_AGENT_SHADOW_STEP, [token, leaseMs * 1000, required(taskId, "Task ID"), stepNo]);
    if (result.affectedRows === 1) {
      return { outcome: "claimed", token };
    }
    const [rows] = await this.pool.execute<ExistingStepRow[]>(GET_AGENT_SHADOW_STEP, [taskId, stepNo]);
    const row = rows[0];
    if (row === undefined) {
      throw new Error(`Agent shadow Step ${taskId}/${stepNo} is missing`);
    }
    return row.status === "completed" ? { outcome: "completed" } : { outcome: "busy" };
  }

  async completeStep(taskId: string, stepNo: number, token: string, output: unknown): Promise<void> {
    const [result] = await this.pool.execute<ResultSetHeader>(COMPLETE_AGENT_SHADOW_STEP, [
      canonicalJSON(output), required(taskId, "Task ID"), stepNo, required(token, "Step claim token")
    ]);
    if (result.affectedRows !== 1) {
      throw new Error(`Agent shadow Step ${taskId}/${stepNo} completion is stale`);
    }
  }

  async failStep(taskId: string, stepNo: number, token: string, error: unknown): Promise<void> {
    const message = (error instanceof Error ? error.message : String(error)).slice(0, 65_535);
    const [result] = await this.pool.execute<ResultSetHeader>(FAIL_AGENT_SHADOW_STEP, [
      message, required(taskId, "Task ID"), stepNo, required(token, "Step claim token")
    ]);
    if (result.affectedRows !== 1) {
      throw new Error(`Agent shadow Step ${taskId}/${stepNo} failure is stale`);
    }
  }
}

function isDuplicateKey(error: unknown): boolean {
  return typeof error === "object" && error !== null && "code" in error && error.code === "ER_DUP_ENTRY";
}

function required(value: string, label: string): string {
  value = value.trim();
  if (!value) {
    throw new Error(`Agent shadow ${label} is required`);
  }
  return value;
}

function canonicalJSON(value: unknown): string {
  return JSON.stringify(canonicalValue(value));
}

function canonicalValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(canonicalValue);
  }
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).sort(([left], [right]) => left.localeCompare(right)).map(([key, item]) => [key, canonicalValue(item)]));
  }
  return value;
}
