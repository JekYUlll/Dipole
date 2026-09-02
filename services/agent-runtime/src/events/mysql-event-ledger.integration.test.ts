import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";

import { createPool, type Pool } from "mysql2/promise";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { MySQLEventLedger } from "./mysql-event-ledger.js";
import {
  createEventLeaseReclaimEvidence,
  parseEventLeaseReclaimEvidence
} from "../runtime/event-lease-reclaim-evidence.js";

const adminUrl = process.env.DIPOLE_TEST_AGENT_MYSQL_URL;
const integration = describe.skipIf(adminUrl === undefined);

integration("MySQLEventLedger MySQL 8.4 contract", () => {
  const database = `dipole_agent_${randomUUID().replaceAll("-", "")}`;
  let admin: Pool;
  let pool: Pool;

  beforeAll(async () => {
    admin = createPool({ uri: adminUrl!, timezone: "Z", connectionLimit: 4 });
    await admin.query(`CREATE DATABASE \`${database}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`);
    const parsed = new URL(adminUrl!);
    parsed.pathname = `/${database}`;
    pool = createPool({ uri: parsed.toString(), timezone: "Z", connectionLimit: 20 });
    const migration = await readFile(new URL("../../../../db/migrations/000018_agent_event_ledger.up.sql", import.meta.url), "utf8");
    await pool.query(migration);
  });

  afterAll(async () => {
    await pool?.end();
    if (admin !== undefined) {
      await admin.query(`DROP DATABASE IF EXISTS \`${database}\``);
      await admin.end();
    }
  });

  it("allows one concurrent owner and keeps completed events idempotent", async () => {
    const ledger = new MySQLEventLedger(pool, 60_000);
    const claims = await Promise.all(Array.from({ length: 16 }, () => ledger.claim("E-CONCURRENT", "TASK-CONCURRENT", "message.direct.created")));
    const winners = claims.filter((claim) => claim !== undefined);

    expect(winners).toHaveLength(1);
    await ledger.complete(winners[0]!);
    await expect(ledger.claim("E-CONCURRENT", "TASK-CONCURRENT", "message.direct.created")).resolves.toBeUndefined();
    await expect(ledger.claim("E-OTHER", "TASK-CONCURRENT", "message.direct.created")).rejects.toThrow(/binding conflict/);
  });

  it("records failure, reclaims a released lease, and rejects the stale owner", async () => {
    const ledger = new MySQLEventLedger(pool, 60_000);
    const first = await ledger.claim("E-RETRY", "TASK-RETRY", "message.direct.created");
    expect(first).toBeDefined();
    await ledger.release(first!, new Error("temporary planner failure"));

    const [failedRows] = await pool.query<Array<{ last_error: string; attempt_count: number } & import("mysql2").RowDataPacket>>(
      "SELECT last_error, attempt_count FROM agent_event_ledger WHERE event_id = 'E-RETRY'"
    );
    expect(failedRows[0]).toMatchObject({ last_error: "temporary planner failure", attempt_count: 1 });

    const second = await ledger.claim("E-RETRY", "TASK-RETRY", "message.direct.created");
    expect(second?.token).not.toBe(first?.token);
    await expect(ledger.complete(first!)).rejects.toThrow(/stale/);
    await ledger.complete(second!);
  });

  it("reclaims an expired lease after a worker crash", async () => {
    const ledger = new MySQLEventLedger(pool, 60_000);
    const crashed = await ledger.claim("E-CRASH", "TASK-CRASH", "message.direct.created");
    expect(crashed).toBeDefined();
    await pool.execute("UPDATE agent_event_ledger SET lease_expires_at = DATE_SUB(UTC_TIMESTAMP(3), INTERVAL 1 SECOND) WHERE event_id = ?", ["E-CRASH"]);

    const recovered = await ledger.claim("E-CRASH", "TASK-CRASH", "message.direct.created");
    expect(recovered).toBeDefined();
    expect(recovered?.token).not.toBe(crashed?.token);
    await expect(ledger.complete(crashed!)).rejects.toThrow(/stale/);
    await ledger.complete(recovered!);

    const [rows] = await pool.query<Array<{ attempt_count: number; status: string } & import("mysql2").RowDataPacket>>(
      "SELECT attempt_count, status FROM agent_event_ledger WHERE event_id = 'E-CRASH'"
    );
    expect(rows).toEqual([{ attempt_count: 2, status: "completed" }]);
    const evidence = createEventLeaseReclaimEvidence({
      event_id: "E-CRASH",
      task_id: "TASK-CRASH",
      expired_claim_reclaimed: true,
      stale_owner_completion_rejected: true,
      reclaim_attempt_count: 2,
      completed_event_count: 1
    });
    expect(parseEventLeaseReclaimEvidence(evidence)).toEqual(evidence);
  });
});
