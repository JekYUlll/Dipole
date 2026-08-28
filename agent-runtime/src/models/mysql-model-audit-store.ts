import { createHash, randomUUID } from "node:crypto";

import type { Pool, PoolConnection, ResultSetHeader, RowDataPacket } from "mysql2/promise";

import type { ModelAuditStore, ModelCallRecovery, ModelCallReservation, ModelRunBudgetPolicy, ModelUsage } from "./model-router.js";
import {
  ABANDON_AGENT_MODEL_CALLS,
  COMPLETE_AGENT_MODEL_CALL,
  COMPLETE_AGENT_MODEL_RUN,
  FAIL_AGENT_MODEL_CALL,
  FAIL_AGENT_MODEL_RUN,
  FAIL_AGENT_MODEL_RUN_BY_TASK,
  INCREMENT_AGENT_MODEL_RUN_CALLS,
  INSERT_AGENT_MODEL_CALL,
  INSERT_AGENT_MODEL_RUN,
  GET_AGENT_MODEL_RUN_STATUS,
  GET_COMPLETED_AGENT_MODEL_CALL,
  LOCK_AGENT_MODEL_RUN
} from "./mysql-model-audit-queries.js";

interface ModelRunRow extends RowDataPacket {
  run_uuid: string;
  task_uuid: string;
  status: "running" | "completed" | "failed";
  max_calls: number;
  total_timeout_ms: number;
  max_output_tokens_per_call: number;
  calls_reserved: number;
}

interface CompletedModelCallRow extends RowDataPacket {
  run_uuid: string;
  call_uuid: string;
  call_no: number;
  route: string;
  output_json: unknown;
  input_tokens: number | null;
  output_tokens: number | null;
  max_calls: number;
  total_timeout_ms: number;
  max_output_tokens_per_call: number;
}

interface ModelRunStatusRow extends RowDataPacket {
  status: "running" | "completed" | "failed";
}

export class MySQLModelAuditStore implements ModelAuditStore {
  constructor(private readonly pool: Pool) {}

  async recover(taskId: string, policy: ModelRunBudgetPolicy): Promise<ModelCallRecovery | undefined> {
    taskId = required(taskId, "Task ID", 64);
    policy = validatePolicy(policy);
    const [rows] = await this.pool.execute<CompletedModelCallRow[]>(GET_COMPLETED_AGENT_MODEL_CALL, [taskId]);
    const row = rows[0];
    if (row === undefined) {
      return undefined;
    }
    if (!samePolicy(row, policy)) {
      throw new Error(`Agent model run policy conflict for ${taskId}`);
    }
    return {
      runId: row.run_uuid,
      callId: row.call_uuid,
      callNo: row.call_no,
      route: row.route,
      output: decodedJSON(row.output_json),
      usage: {
        inputTokens: row.input_tokens ?? undefined,
        outputTokens: row.output_tokens ?? undefined
      }
    };
  }

  async reserve(taskId: string, policy: ModelRunBudgetPolicy, route: string): Promise<ModelCallReservation | undefined> {
    taskId = required(taskId, "Task ID", 64);
    route = required(route, "model route", 255);
    policy = validatePolicy(policy);
    const runId = agentModelRunId(taskId);
    await this.pool.execute(INSERT_AGENT_MODEL_RUN, [
      runId, taskId, policy.maxCalls, policy.totalTimeoutMs, policy.maxOutputTokensPerCall
    ]);
    const connection = await this.pool.getConnection();
    try {
      await connection.beginTransaction();
      const row = await lockRun(connection, taskId);
      if (row.run_uuid !== runId || row.task_uuid !== taskId) {
        throw new Error(`Agent model run binding conflict for ${taskId}`);
      }
      if (!samePolicy(row, policy)) {
        throw new Error(`Agent model run policy conflict for ${taskId}`);
      }
      if (row.status !== "running" || row.calls_reserved >= row.max_calls) {
        await connection.commit();
        return undefined;
      }

      const callNo = row.calls_reserved + 1;
      const callId = randomUUID();
      const [incremented] = await connection.execute<ResultSetHeader>(INCREMENT_AGENT_MODEL_RUN_CALLS, [runId]);
      requireOne(incremented, `reserve model call ${callNo}`);
      await connection.execute(INSERT_AGENT_MODEL_CALL, [callId, runId, callNo, route]);
      await connection.commit();
      return { runId, callId, callNo, route };
    } catch (error) {
      await connection.rollback().catch(() => undefined);
      throw error;
    } finally {
      connection.release();
    }
  }

  async completeCall(
    reservation: ModelCallReservation,
    output: unknown,
    usage: ModelUsage,
    finishReason: string,
    latencyMs: number
  ): Promise<void> {
    finishReason = required(finishReason, "finish reason", 64);
    latencyMs = unsignedInteger(latencyMs, "latency");
    const inputTokens = optionalUnsignedInteger(usage.inputTokens, "input tokens");
    const outputTokens = optionalUnsignedInteger(usage.outputTokens, "output tokens");
    const outputJSON = JSON.stringify(output);
    if (outputJSON === undefined) {
      throw new Error("Agent model output must be JSON serializable");
    }
    const [result] = await this.pool.execute<ResultSetHeader>(COMPLETE_AGENT_MODEL_CALL, [
      outputJSON, inputTokens, outputTokens, finishReason, latencyMs,
      reservation.callId, reservation.runId, reservation.callNo
    ]);
    requireOne(result, `complete model call ${reservation.callId}`);
  }

  async failCall(reservation: ModelCallReservation, error: unknown, latencyMs: number): Promise<void> {
    latencyMs = unsignedInteger(latencyMs, "latency");
    const [result] = await this.pool.execute<ResultSetHeader>(FAIL_AGENT_MODEL_CALL, [
      latencyMs, errorText(error), reservation.callId, reservation.runId, reservation.callNo
    ]);
    requireOne(result, `fail model call ${reservation.callId}`);
  }

  async completeRun(runId: string): Promise<void> {
    await this.finishRun(runId, "completed");
  }

  async failRun(runId: string, error: unknown): Promise<void> {
    await this.finishRun(runId, "failed", errorText(error));
  }

  async failTask(taskId: string, error: unknown): Promise<void> {
    taskId = required(taskId, "Task ID", 64);
    await this.pool.execute(FAIL_AGENT_MODEL_RUN_BY_TASK, [errorText(error), taskId]);
  }

  private async finishRun(runId: string, status: "completed" | "failed", error: string | null = null): Promise<void> {
    runId = required(runId, "Run ID", 64);
    const connection = await this.pool.getConnection();
    try {
      await connection.beginTransaction();
      const abandonedReason = status === "completed" ? "run completed before call terminal" : (error ?? "run failed");
      await connection.execute(ABANDON_AGENT_MODEL_CALLS, [abandonedReason, runId]);
      const [result] = status === "completed"
        ? await connection.execute<ResultSetHeader>(COMPLETE_AGENT_MODEL_RUN, [runId])
        : await connection.execute<ResultSetHeader>(FAIL_AGENT_MODEL_RUN, [error, runId]);
      if (result.affectedRows !== 1) {
        const [rows] = await connection.execute<ModelRunStatusRow[]>(GET_AGENT_MODEL_RUN_STATUS, [runId]);
        if (rows[0]?.status !== status) {
          throw new Error(`Agent model audit state is stale: cannot ${status} model run ${runId}`);
        }
      }
      await connection.commit();
    } catch (cause) {
      await connection.rollback().catch(() => undefined);
      throw cause;
    } finally {
      connection.release();
    }
  }
}

function decodedJSON(value: unknown): unknown {
  if (typeof value === "string") {
    return JSON.parse(value);
  }
  return value;
}

export function agentModelRunId(taskId: string): string {
  taskId = required(taskId, "Task ID", 64);
  const digest = createHash("sha256").update(`dipole.agent.model.run.v1\n${taskId}`, "utf8").digest("hex");
  return `run:${digest.slice(0, 59)}`;
}

async function lockRun(connection: PoolConnection, taskId: string): Promise<ModelRunRow> {
  const [rows] = await connection.execute<ModelRunRow[]>(LOCK_AGENT_MODEL_RUN, [taskId]);
  if (rows.length !== 1) {
    throw new Error(`Agent model run missing or duplicated for ${taskId}`);
  }
  return rows[0]!;
}

function samePolicy(
  row: Pick<ModelRunRow, "max_calls" | "total_timeout_ms" | "max_output_tokens_per_call">,
  policy: ModelRunBudgetPolicy
): boolean {
  return row.max_calls === policy.maxCalls
    && row.total_timeout_ms === policy.totalTimeoutMs
    && row.max_output_tokens_per_call === policy.maxOutputTokensPerCall;
}

function validatePolicy(policy: ModelRunBudgetPolicy): ModelRunBudgetPolicy {
  return {
    maxCalls: positiveInteger(policy.maxCalls, "max calls", 65_535),
    totalTimeoutMs: positiveInteger(policy.totalTimeoutMs, "total timeout", 4_294_967_295),
    maxOutputTokensPerCall: positiveInteger(policy.maxOutputTokensPerCall, "max output tokens", 4_294_967_295)
  };
}

function positiveInteger(value: number, label: string, maximum: number): number {
  if (!Number.isSafeInteger(value) || value < 1 || value > maximum) {
    throw new Error(`Agent model ${label} must be an integer between 1 and ${maximum}`);
  }
  return value;
}

function unsignedInteger(value: number, label: string): number {
  if (!Number.isSafeInteger(value) || value < 0 || value > 4_294_967_295) {
    throw new Error(`Agent model ${label} must be an unsigned 32-bit integer`);
  }
  return value;
}

function optionalUnsignedInteger(value: number | undefined, label: string): number | null {
  return value === undefined ? null : unsignedInteger(value, label);
}

function required(value: string, label: string, maxLength: number): string {
  value = value.trim();
  if (!value || value.length > maxLength) {
    throw new Error(`Agent model ${label} must contain 1-${maxLength} characters`);
  }
  return value;
}

function requireOne(result: ResultSetHeader, operation: string): void {
  if (result.affectedRows !== 1) {
    throw new Error(`Agent model audit state is stale: cannot ${operation}`);
  }
}

function errorText(error: unknown): string | null {
  if (error === undefined || error === null) {
    return null;
  }
  return (error instanceof Error ? error.message : String(error)).slice(0, 65_535);
}
