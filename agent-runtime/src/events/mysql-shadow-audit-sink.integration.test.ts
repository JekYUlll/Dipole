import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";

import { createPool, type Pool, type RowDataPacket } from "mysql2/promise";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { memoryContextReferences, MySQLShadowAuditSink } from "./mysql-shadow-audit-sink.js";
import { MySQLMemoryDerivedLineageStore } from "../memory/memory-derived-lineage.js";

describe("Memory Context lineage extraction", () => {
  it("sorts and deduplicates exact Memory references", () => {
    expect(memoryContextReferences({
      compilerVersion: "v1", estimatedTokens: 20, omitted: [], selected: [
        { id: "memory:MEM-2", representation: "compact", provenance: { sourceType: "message", sourceId: "M2" } },
        { id: "event:E1", representation: "full", provenance: { sourceType: "event", sourceId: "E1" } },
        { id: "memory:MEM-1", representation: "full", provenance: { sourceType: "message", sourceId: "M1" } },
        { id: "memory:MEM-2", representation: "compact", provenance: { sourceType: "message", sourceId: "M2" } }
      ]
    })).toEqual([{ memoryId: "MEM-1", representation: "full" }, { memoryId: "MEM-2", representation: "compact" }]);
  });

  it("fails closed on invalid or conflicting Memory references", () => {
    const context = (id: string, representation: "full" | "compact") => ({
      compilerVersion: "v1" as const, estimatedTokens: 10, omitted: [],
      selected: [{ id, representation, provenance: { sourceType: "message", sourceId: "M1" } }]
    });
    expect(() => memoryContextReferences(context("memory:bad/id", "full"))).toThrow(/identity/);
    expect(() => memoryContextReferences({
      ...context("memory:MEM-1", "full"),
      selected: [...context("memory:MEM-1", "full").selected, ...context("memory:MEM-1", "compact").selected]
    })).toThrow(/representation/);
  });
});

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
    await pool.query(await readFile(new URL("../../../db/migrations/000016_agent_policy_persistence.up.sql", import.meta.url), "utf8"));
    await pool.query(await readFile(new URL("../../../db/migrations/000019_agent_model_audit.up.sql", import.meta.url), "utf8"));
    const migration = await readFile(new URL("../../../db/migrations/000020_agent_shadow_trajectory.up.sql", import.meta.url), "utf8");
    await pool.query(migration);
    const runMigration = await readFile(new URL("../../../db/migrations/000021_agent_runs.up.sql", import.meta.url), "utf8");
    await pool.query("SET FOREIGN_KEY_CHECKS=0");
    await pool.query(runMigration);
    await pool.query("SET FOREIGN_KEY_CHECKS=1");
    const contextMigration = await readFile(new URL("../../../db/migrations/000022_agent_context_manifest.up.sql", import.meta.url), "utf8");
    await pool.query(contextMigration);
    for (const migrationFile of ["000023_agent_model_output_replay", "000026_agent_artifacts", "000030_agent_tool_invocations", "000031_agent_tool_action_lineage"]) {
      await pool.query(await readFile(new URL(`../../../db/migrations/${migrationFile}.up.sql`, import.meta.url), "utf8"));
    }
    for (const version of [29, 38, 39, 40, 41]) {
      const name = ({ 29: "agent_memories", 38: "agent_memory_owner_governance", 39: "agent_memory_corrections", 40: "agent_memory_content_erasure", 41: "agent_memory_task_lineage" } as const)[version as 29 | 38 | 39 | 40 | 41];
      await pool.query(await readFile(new URL(`../../../db/migrations/${String(version).padStart(6, "0")}_${name}.up.sql`, import.meta.url), "utf8"));
    }
    await pool.query("INSERT INTO agent_memories (memory_uuid, tenant_id, principal_uuid, agent_uuid, memory_type, status, resource_type, resource_id, content, priority, source_type, source_id, valid_from, memory_root_uuid, memory_version) VALUES ('MEM-PLAN-1', 'dipole', 'U100', 'UAI', 'semantic', 'active', 'conversation', 'group:G1', 'private fact', 80, 'message', 'M1', UTC_TIMESTAMP(3), 'MEM-PLAN-1', 1)");
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
            }, {
              id: "memory:MEM-PLAN-1", representation: "compact" as const,
              provenance: { sourceType: "message", sourceId: "M1" }
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
    const [lineage] = await pool.query<Array<RowDataPacket & { memory_uuid: string; task_uuid: string; representation: string; source: string }>>(
      "SELECT memory_uuid, task_uuid, representation, source FROM agent_memory_task_lineage WHERE task_uuid = ?", [record.taskId]
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
    expect(lineage).toEqual([{ memory_uuid: "MEM-PLAN-1", task_uuid: record.taskId, representation: "compact", source: "runtime_write" }]);
    await expect(new MySQLMemoryDerivedLineageStore(pool).load({
      schemaVersion: "dipole.agent.memory-derived-lineage-manifest.v1", tenantId: "dipole", principalId: "U100", memoryId: "MEM-PLAN-1"
    })).resolves.toMatchObject({
      lineageVersions: 1, directTaskReferences: 1, unindexedContextPlans: 0, unattributedModelTasks: 0, lineageComplete: true,
      domains: { modelCalls: 0, shadowPlans: 1, shadowSteps: 2, artifacts: 0, toolInvocations: 0, messageActions: 0, temporalHistoryPotentialTasks: 1 },
      contentRead: false, deletionAuthority: false, runtimeAuthority: false
    });
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

  it("finds exact historical Context references without underscore wildcard matches", async () => {
    await pool.query("INSERT INTO agent_memories (memory_uuid, tenant_id, principal_uuid, agent_uuid, memory_type, status, resource_type, resource_id, content, priority, source_type, source_id, valid_from, memory_root_uuid, memory_version) VALUES ('MEM_A', 'dipole', 'U100', 'UAI', 'semantic', 'active', 'conversation', 'group:G1', 'legacy fact', 80, 'message', 'M2', UTC_TIMESTAMP(3), 'MEM_A', 1)");
    const insertLegacyPlan = async (taskId: string, eventId: string, memoryId: string) => pool.query(
      "INSERT INTO agent_shadow_plans (task_uuid, event_id, event_type, summary, plan_sha256, context_compiler_version, context_estimated_tokens, context_manifest_json) VALUES (?, ?, 'message.direct.created', 'legacy', ?, 'v1', 1, ?)",
      [taskId, eventId, "a".repeat(64), JSON.stringify({ selected: [{ id: `memory:${memoryId}`, representation: "full" }], omitted: [] })]
    );
    await insertLegacyPlan("TASK-LEGACY-EXACT", "E-LEGACY-EXACT", "MEM_A");
    await insertLegacyPlan("TASK-LEGACY-SIMILAR", "E-LEGACY-SIMILAR", "MEMXA");

    await expect(new MySQLMemoryDerivedLineageStore(pool).load({
      schemaVersion: "dipole.agent.memory-derived-lineage-manifest.v1", tenantId: "dipole", principalId: "U100", memoryId: "MEM_A"
    })).resolves.toMatchObject({
      directTaskReferences: 0,
      unindexedContextPlans: 1,
      lineageComplete: false,
      unattributedModelTasks: 0,
      domains: { temporalHistoryPotentialTasks: 1 }
    });
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

  it("marks completed Model output without a Plan as an unattributed completeness gap", async () => {
    await pool.query("INSERT INTO agent_tasks (task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid, status, trigger_type, trigger_ref, goal) VALUES ('TASK-ORPHAN-41', 'DEF-ORPHAN-41', 1, 'dipole', 'U100', 'UAI', 'running', 'message.direct.created', 'M-ORPHAN-41', 'audit crash gap')");
    await pool.query("INSERT INTO agent_model_runs (run_uuid, task_uuid, status, max_calls, total_timeout_ms, max_output_tokens_per_call, calls_reserved, started_at, completed_at) VALUES ('RUN-ORPHAN-41', 'TASK-ORPHAN-41', 'completed', 1, 1000, 100, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))");
    await pool.query("INSERT INTO agent_model_calls (call_uuid, run_uuid, call_no, route, status, output_json, input_tokens, output_tokens, finish_reason, latency_ms, started_at, finished_at) VALUES ('00000000-0000-0000-0000-000000000041', 'RUN-ORPHAN-41', 1, 'gateway/primary', 'completed', '{\"summary\":\"derived\"}', 10, 2, 'stop', 10, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))");
    await pool.query("INSERT INTO agent_tasks (task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid, status, trigger_type, trigger_ref, goal) VALUES ('TASK-ORPHAN-42', 'DEF-ORPHAN-42', 1, 'dipole', 'U200', 'UAI', 'running', 'message.direct.created', 'M-ORPHAN-42', 'foreign audit gap')");
    await pool.query("INSERT INTO agent_model_runs (run_uuid, task_uuid, status, max_calls, total_timeout_ms, max_output_tokens_per_call, calls_reserved, started_at, completed_at) VALUES ('RUN-ORPHAN-42', 'TASK-ORPHAN-42', 'completed', 1, 1000, 100, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))");
    await pool.query("INSERT INTO agent_model_calls (call_uuid, run_uuid, call_no, route, status, output_json, input_tokens, output_tokens, finish_reason, latency_ms, started_at, finished_at) VALUES ('00000000-0000-0000-0000-000000000042', 'RUN-ORPHAN-42', 1, 'gateway/primary', 'completed', '{\"summary\":\"foreign\"}', 10, 2, 'stop', 10, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))");
    await expect(new MySQLMemoryDerivedLineageStore(pool).load({
      schemaVersion: "dipole.agent.memory-derived-lineage-manifest.v1", tenantId: "dipole", principalId: "U100", memoryId: "MEM-PLAN-1"
    })).resolves.toMatchObject({
      directTaskReferences: 1,
      unattributedModelTasks: 1,
      lineageComplete: false
    });
  });
});
