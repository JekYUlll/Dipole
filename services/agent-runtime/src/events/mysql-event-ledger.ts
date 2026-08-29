import { randomUUID } from "node:crypto";

import type { Pool, PoolConnection, ResultSetHeader, RowDataPacket } from "mysql2/promise";

import type { EventClaim, EventLedger } from "./event-ledger.js";
import {
  COMPLETE_AGENT_EVENT,
  INSERT_AGENT_EVENT_CLAIM,
  LOCK_AGENT_EVENT_CLAIM,
  RECLAIM_AGENT_EVENT,
  RELEASE_AGENT_EVENT
} from "./mysql-event-ledger-queries.js";

interface LedgerRow extends RowDataPacket {
  id: string;
  event_id: string;
  task_uuid: string;
  status: "claimed" | "completed";
  claim_token: string;
  lease_expires_at: Date;
}

export class MySQLEventLedger implements EventLedger {
  constructor(private readonly pool: Pool, private readonly leaseMs = 60_000) {
    if (!Number.isSafeInteger(leaseMs) || leaseMs < 1000 || leaseMs > 86_400_000) {
      throw new Error("Agent EventLedger lease must be between 1 second and 24 hours");
    }
  }

  async claim(eventId: string, taskId: string, eventType = "unknown"): Promise<EventClaim | undefined> {
    eventId = required(eventId, "event ID");
    taskId = required(taskId, "Task ID");
    eventType = required(eventType, "event type");
    const token = randomUUID();
    const connection = await this.pool.getConnection();
    try {
      await connection.beginTransaction();
      const inserted = await this.tryInsert(connection, { eventId, taskId, eventType, token });
      if (inserted) {
        await connection.commit();
        return { eventId, taskId, token };
      }

      const row = await this.lockExisting(connection, eventId, taskId);
      if (row === undefined || row.event_id !== eventId || row.task_uuid !== taskId) {
        throw new Error(`event ledger binding conflict for ${eventId}`);
      }
      if (row.status === "completed" || row.lease_expires_at.getTime() > Date.now()) {
        await connection.commit();
        return undefined;
      }

      const [result] = await connection.execute<ResultSetHeader>(
        RECLAIM_AGENT_EVENT,
        [token, this.leaseMs * 1000, row.id, row.claim_token]
      );
      if (result.affectedRows !== 1) {
        throw new Error(`event ledger claim changed concurrently for ${eventId}`);
      }
      await connection.commit();
      return { eventId, taskId, token };
    } catch (error) {
      await connection.rollback().catch(() => undefined);
      throw error;
    } finally {
      connection.release();
    }
  }

  async complete(claim: EventClaim): Promise<void> {
    const [result] = await this.pool.execute<ResultSetHeader>(
      COMPLETE_AGENT_EVENT,
      [claim.eventId, claim.taskId, claim.token]
    );
    requireOne(result, claim.eventId);
  }

  async release(claim: EventClaim, error?: unknown): Promise<void> {
    const lastError = errorText(error);
    const [result] = await this.pool.execute<ResultSetHeader>(
      RELEASE_AGENT_EVENT,
      [lastError, claim.eventId, claim.taskId, claim.token]
    );
    requireOne(result, claim.eventId);
  }

  private async tryInsert(connection: PoolConnection, input: { eventId: string; taskId: string; eventType: string; token: string }): Promise<boolean> {
    try {
      await connection.execute(
        INSERT_AGENT_EVENT_CLAIM,
        [input.eventId, input.taskId, input.eventType, input.token, this.leaseMs * 1000]
      );
      return true;
    } catch (error) {
      if (isDuplicateKey(error)) {
        return false;
      }
      throw error;
    }
  }

  private async lockExisting(connection: PoolConnection, eventId: string, taskId: string): Promise<LedgerRow | undefined> {
    const [rows] = await connection.execute<LedgerRow[]>(
      LOCK_AGENT_EVENT_CLAIM,
      [eventId, taskId]
    );
    if (rows.length > 1) {
      throw new Error(`event ledger has conflicting bindings for ${eventId}`);
    }
    return rows[0];
  }
}

function required(value: string, label: string): string {
  value = value.trim();
  if (!value) {
    throw new Error(`Agent EventLedger ${label} is required`);
  }
  return value;
}

function isDuplicateKey(error: unknown): boolean {
  return typeof error === "object" && error !== null && "code" in error && error.code === "ER_DUP_ENTRY";
}

function requireOne(result: ResultSetHeader, eventId: string): void {
  if (result.affectedRows !== 1) {
    throw new Error(`event ledger claim is stale for ${eventId}`);
  }
}

function errorText(error: unknown): string | null {
  if (error === undefined) {
    return null;
  }
  const value = error instanceof Error ? error.message : String(error);
  return value.slice(0, 65_535);
}
