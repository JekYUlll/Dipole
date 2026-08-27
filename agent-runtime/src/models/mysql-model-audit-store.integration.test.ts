import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";

import { createPool, type Pool, type RowDataPacket } from "mysql2/promise";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { MySQLModelAuditStore } from "./mysql-model-audit-store.js";

const adminUrl = process.env.DIPOLE_TEST_AGENT_MYSQL_URL;
const integration = describe.skipIf(adminUrl === undefined);
const policy = { maxCalls: 3, totalTimeoutMs: 15_000, maxOutputTokensPerCall: 512 };

integration("MySQLModelAuditStore MySQL 8.4 contract", () => {
  const database = `dipole_agent_model_${randomUUID().replaceAll("-", "")}`;
  let admin: Pool;
  let pool: Pool;

  beforeAll(async () => {
    admin = createPool({ uri: adminUrl!, timezone: "Z", connectionLimit: 4 });
    await admin.query(`CREATE DATABASE \`${database}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`);
    const parsed = new URL(adminUrl!);
    parsed.pathname = `/${database}`;
    pool = createPool({ uri: parsed.toString(), timezone: "Z", connectionLimit: 20, multipleStatements: true });
    const migration = await readFile(new URL("../../../db/migrations/000019_agent_model_audit.up.sql", import.meta.url), "utf8");
    await pool.query(migration);
  });

  afterAll(async () => {
    await pool?.end();
    if (admin !== undefined) {
      await admin.query(`DROP DATABASE IF EXISTS \`${database}\``);
      await admin.end();
    }
  });

  it("atomically grants only the configured number of call slots", async () => {
    const store = new MySQLModelAuditStore(pool);
    const reservations = await Promise.all(Array.from({ length: 16 }, () =>
      store.reserve("TASK-CONCURRENT", policy, "gateway/primary")
    ));
    const granted = reservations.filter((reservation) => reservation !== undefined);

    expect(granted).toHaveLength(3);
    expect(granted.map((reservation) => reservation!.callNo).sort()).toEqual([1, 2, 3]);
    expect(new Set(granted.map((reservation) => reservation!.runId))).toHaveProperty("size", 1);
  });

  it("reuses the Task run and rejects policy drift", async () => {
    const store = new MySQLModelAuditStore(pool);
    const first = await store.reserve("TASK-RETRY", policy, "gateway/primary");
    const second = await store.reserve("TASK-RETRY", policy, "gateway/fallback");

    expect(second?.runId).toBe(first?.runId);
    await expect(store.reserve("TASK-RETRY", { ...policy, maxCalls: 4 }, "gateway/other")).rejects.toThrow(/policy conflict/);
  });

  it("records terminal call evidence and rejects a stale completion", async () => {
    const store = new MySQLModelAuditStore(pool);
    const completed = await store.reserve("TASK-TERMINAL", policy, "gateway/primary");
    await store.completeCall(completed!, { inputTokens: 21, outputTokens: 7 }, "stop", 35);
    const failed = await store.reserve("TASK-TERMINAL", policy, "gateway/fallback");
    await store.failCall(failed!, new Error("provider unavailable"), 12);
    await expect(store.completeCall(failed!, { inputTokens: 1, outputTokens: 1 }, "stop", 1)).rejects.toThrow(/stale/);

    const [rows] = await pool.query<Array<RowDataPacket & {
      status: string; input_tokens: number | null; output_tokens: number | null; finish_reason: string | null; last_error: string | null;
    }>>("SELECT status, input_tokens, output_tokens, finish_reason, last_error FROM agent_model_calls WHERE run_uuid = ? ORDER BY call_no", [completed!.runId]);
    expect(rows).toEqual([
      expect.objectContaining({ status: "completed", input_tokens: 21, output_tokens: 7, finish_reason: "stop", last_error: null }),
      expect.objectContaining({ status: "failed", input_tokens: null, output_tokens: null, finish_reason: null, last_error: "provider unavailable" })
    ]);
  });

  it("conserves crashed reservations and blocks calls after run completion", async () => {
    const store = new MySQLModelAuditStore(pool);
    const tight = { ...policy, maxCalls: 2 };
    const crashed = await store.reserve("TASK-CRASH", tight, "gateway/primary");
    const recovered = await store.reserve("TASK-CRASH", tight, "gateway/fallback");

    expect(crashed).toBeDefined();
    expect(recovered).toBeDefined();
    await expect(store.reserve("TASK-CRASH", tight, "gateway/other")).resolves.toBeUndefined();
    await store.completeCall(recovered!, { inputTokens: 5, outputTokens: 2 }, "stop", 8);
    await store.completeRun(recovered!.runId);
    await expect(store.reserve("TASK-CRASH", tight, "gateway/other")).resolves.toBeUndefined();

    const [rows] = await pool.query<Array<RowDataPacket & { call_no: number; status: string }>>(
      "SELECT call_no, status FROM agent_model_calls WHERE run_uuid = ? ORDER BY call_no", [recovered!.runId]
    );
    expect(rows).toEqual([
      expect.objectContaining({ call_no: 1, status: "abandoned" }),
      expect.objectContaining({ call_no: 2, status: "completed" })
    ]);
  });
});
