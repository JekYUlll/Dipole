import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";

import { createPool, type Pool, type RowDataPacket } from "mysql2/promise";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { MySQLShadowAuditSink } from "./mysql-shadow-audit-sink.js";

const adminUrl = process.env.DIPOLE_TEST_AGENT_MYSQL_URL;
const integration = describe.skipIf(adminUrl === undefined);

integration("MySQLShadowAuditSink MySQL 8.4 contract", () => {
  const database = `dipole_agent_steps_${randomUUID().replaceAll("-", "")}`;
  let admin: Pool;
  let pool: Pool;

  beforeAll(async () => {
    admin = createPool({ uri: adminUrl!, timezone: "Z", connectionLimit: 4 });
    await admin.query(`CREATE DATABASE \`${database}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`);
    const parsed = new URL(adminUrl!);
    parsed.pathname = `/${database}`;
    pool = createPool({ uri: parsed.toString(), timezone: "Z", connectionLimit: 10, multipleStatements: true });
    const migration = await readFile(new URL("../../../db/migrations/000020_agent_shadow_trajectory.up.sql", import.meta.url), "utf8");
    await pool.query(migration);
    const runMigration = await readFile(new URL("../../../db/migrations/000021_agent_runs.up.sql", import.meta.url), "utf8");
    await pool.query("SET FOREIGN_KEY_CHECKS=0");
    await pool.query(runMigration);
    await pool.query("SET FOREIGN_KEY_CHECKS=1");
    const contextMigration = await readFile(new URL("../../../db/migrations/000022_agent_context_manifest.up.sql", import.meta.url), "utf8");
    await pool.query(contextMigration);
  });

  afterAll(async () => {
    await pool?.end();
    if (admin !== undefined) {
      await admin.query(`DROP DATABASE IF EXISTS \`${database}\``);
      await admin.end();
    }
  });

  it("persists an immutable structured plan and its ordered steps", async () => {
    const sink = new MySQLShadowAuditSink(pool);
    const record = {
      eventId: "E-PLAN-1", taskId: "TASK-PLAN-1", eventType: "message.direct.created",
      plan: {
        summary: "inspect recent conversations",
        steps: [
          { capabilityId: "conversation.list", input: { limit: 20 } },
          { capabilityId: "conversation.read", input: { conversationId: "group:G1", limit: 50 } }
        ],
        model: {
          route: "gateway/primary", attempts: 1, inputTokens: 30, outputTokens: 12,
          context: {
            compilerVersion: "v1" as const, estimatedTokens: 120,
            selected: [{
              id: "event:E-PLAN-1", representation: "full" as const,
              provenance: { sourceType: "kafka_event", sourceId: "E-PLAN-1" }
            }],
            omitted: []
          }
        }
      }
    } as const;

    await Promise.all(Array.from({ length: 8 }, () => sink.append(record)));

    const [plans] = await pool.query<Array<RowDataPacket & {
      summary: string; model_route: string; model_attempts: number; context_compiler_version: string;
      context_estimated_tokens: number; context_manifest_json: { selected: unknown[]; omitted: unknown[] };
    }>>(
      "SELECT summary, model_route, model_attempts, context_compiler_version, context_estimated_tokens, context_manifest_json FROM agent_shadow_plans WHERE task_uuid = ?",
      [record.taskId]
    );
    const [steps] = await pool.query<Array<RowDataPacket & { step_no: number; capability_id: string; status: string; input_json: unknown }>>(
      "SELECT step_no, capability_id, status, input_json FROM agent_shadow_steps WHERE task_uuid = ? ORDER BY step_no", [record.taskId]
    );
    expect(plans).toEqual([expect.objectContaining({
      summary: record.plan.summary, model_route: "gateway/primary", model_attempts: 1,
      context_compiler_version: "v1", context_estimated_tokens: 120,
      context_manifest_json: { selected: record.plan.model.context.selected, omitted: [] }
    })]);
    expect(steps).toEqual([
      expect.objectContaining({ step_no: 1, capability_id: "conversation.list", status: "planned" }),
      expect.objectContaining({ step_no: 2, capability_id: "conversation.read", status: "planned" })
    ]);
    expect(steps[0]!.input_json).toEqual({ limit: 20 });
  });

  it("rejects policy drift for an existing Task plan", async () => {
    const sink = new MySQLShadowAuditSink(pool);
    const base = {
      eventId: "E-PLAN-2", taskId: "TASK-PLAN-2", eventType: "message.direct.created",
      plan: { summary: "list", steps: [{ capabilityId: "conversation.list", input: { limit: 20 } }] }
    } as const;
    await sink.append(base);

    await expect(sink.append({ ...base, plan: { ...base.plan, summary: "changed" } })).rejects.toThrow(/plan conflict/);
    await expect(sink.append({ ...base, eventType: "message.group.created" })).rejects.toThrow(/plan conflict/);
  });

  it("claims, retries, and terminates a Step with exact ownership", async () => {
    const sink = new MySQLShadowAuditSink(pool);
    const record = {
      eventId: "E-STEP-1", taskId: "TASK-STEP-1", eventType: "message.direct.created",
      plan: { summary: "list", steps: [{ capabilityId: "conversation.list", input: { limit: 20 } }] }
    } as const;
    await sink.append(record);

    const first = await sink.claimStep(record.taskId, 1, 60_000);
    expect(first.outcome).toBe("claimed");
    await expect(sink.claimStep(record.taskId, 1, 60_000)).resolves.toEqual({ outcome: "busy" });
    if (first.outcome !== "claimed") throw new Error("expected first Step owner");
    await sink.failStep(record.taskId, 1, first.token, new Error("temporary"));

    const retry = await sink.claimStep(record.taskId, 1, 60_000);
    if (retry.outcome !== "claimed") throw new Error("expected retry Step owner");
    await expect(sink.completeStep(record.taskId, 1, first.token, { stale: true })).rejects.toThrow(/stale/);
    await sink.completeStep(record.taskId, 1, retry.token, { conversations: [] });
    await expect(sink.claimStep(record.taskId, 1, 60_000)).resolves.toEqual({ outcome: "completed" });
  });
});
