import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";

import { createPool, type Pool } from "mysql2/promise";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { MySQLShadowEvalObservationStore } from "./mysql-shadow-eval-store.js";

const adminUrl = process.env.DIPOLE_TEST_AGENT_MYSQL_URL;
const integration = describe.skipIf(adminUrl === undefined);

integration("MySQL Shadow evaluation observation MySQL 8.4 contract", () => {
  const database = `dipole_agent_eval_${randomUUID().replaceAll("-", "")}`;
  let admin: Pool;
  let pool: Pool;

  beforeAll(async () => {
    admin = createPool({ uri: adminUrl!, timezone: "Z", connectionLimit: 4 });
    await admin.query(`CREATE DATABASE \`${database}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`);
    const parsed = new URL(adminUrl!);
    parsed.pathname = `/${database}`;
    pool = createPool({ uri: parsed.toString(), timezone: "Z", connectionLimit: 4, multipleStatements: true });
    for (const migration of [16, 17, 19, 20, 21, 22, 23, 26, 30, 32, 54]) {
      const prefix = migration.toString().padStart(6, "0");
      const [path] = migrationPaths.filter(item => item.includes(`/${prefix}_`));
      if (path === undefined) throw new Error(`missing Agent migration ${prefix}`);
      await pool.query(await readFile(new URL(path, import.meta.url), "utf8"));
    }
    await seed(pool);
  });

  afterAll(async () => {
    await pool?.end();
    if (admin !== undefined) {
      await admin.query(`DROP DATABASE IF EXISTS \`${database}\``);
      await admin.end();
    }
  });

  it("reads the complete bounded observation from persisted audit tables", async () => {
    const store = new MySQLShadowEvalObservationStore(pool);

    await expect(store.load("TASK-EVAL-1", "RUN-EVAL-1")).resolves.toEqual({
      taskId: "TASK-EVAL-1", taskStatus: "completed", runId: "RUN-EVAL-1", runStatus: "completed", traceId: "trace:eval-1",
      contextManifest: {
        selected: [{ id: "event:E1", provenance: { sourceType: "kafka_event", sourceId: "E1" } }], omitted: []
      },
      steps: [{ stepNo: 1, capabilityId: "conversation.list", status: "completed", attemptCount: 1, latencyMs: 20 }],
      artifacts: [{ artifactType: "conversation_digest", version: 1 }],
      modelCalls: [{ route: "gateway/primary", status: "completed", inputTokens: 12, outputTokens: 3, latencyMs: 40 }],
      toolCalls: [{ status: "completed", latencyMs: 5 }]
    });
  });
});

const migrationPaths = [
  "../../../../db/migrations/000016_agent_policy_persistence.up.sql",
  "../../../../db/migrations/000017_agent_policy_identity_width.up.sql",
  "../../../../db/migrations/000019_agent_model_audit.up.sql",
  "../../../../db/migrations/000020_agent_shadow_trajectory.up.sql",
  "../../../../db/migrations/000021_agent_runs.up.sql",
  "../../../../db/migrations/000022_agent_context_manifest.up.sql",
  "../../../../db/migrations/000023_agent_model_output_replay.up.sql",
  "../../../../db/migrations/000026_agent_artifacts.up.sql",
  "../../../../db/migrations/000030_agent_tool_invocations.up.sql",
  "../../../../db/migrations/000032_agent_runtime_promotion_grants.up.sql",
  "../../../../db/migrations/000054_agent_run_trace_correlation.up.sql"
];

async function seed(pool: Pool): Promise<void> {
  await pool.execute(
    "INSERT INTO agent_tasks (task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid, status, trigger_type, trigger_ref, goal) VALUES (?, ?, 1, 'dipole', 'U1', 'AI1', 'completed', 'event', 'E1', 'evaluate')",
    ["TASK-EVAL-1", "DEF-EVAL-1"]
  );
  await pool.execute(
    "INSERT INTO agent_runs (run_uuid, task_uuid, runtime_id, trace_id, mode, status, started_at, completed_at) VALUES ('RUN-EVAL-1', 'TASK-EVAL-1', 'runtime-1', 'trace:eval-1', 'shadow', 'completed', UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))"
  );
  await pool.execute(
    "INSERT INTO agent_shadow_plans (task_uuid, event_id, event_type, summary, plan_sha256, context_compiler_version, context_estimated_tokens, context_manifest_json) VALUES ('TASK-EVAL-1', 'E1', 'message.direct.created', 'summary', ?, 'v1', 10, ?)",
    ["a".repeat(64), JSON.stringify({ selected: [{ id: "event:E1", provenance: { sourceType: "kafka_event", sourceId: "E1" } }], omitted: [] })]
  );
  await pool.execute(
    "INSERT INTO agent_shadow_steps (task_uuid, step_no, capability_id, status, input_json, output_json, attempt_count, started_at, finished_at) VALUES ('TASK-EVAL-1', 1, 'conversation.list', 'completed', JSON_OBJECT(), JSON_OBJECT(), 1, '2026-08-27 00:00:00.000', '2026-08-27 00:00:00.020')"
  );
  await pool.execute(
    "INSERT INTO agent_model_runs (run_uuid, task_uuid, status, max_calls, total_timeout_ms, max_output_tokens_per_call, calls_reserved, started_at, completed_at) VALUES ('MODEL-RUN-1', 'TASK-EVAL-1', 'completed', 1, 1000, 100, 1, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))"
  );
  await pool.execute(
    "INSERT INTO agent_model_calls (call_uuid, run_uuid, call_no, route, status, input_tokens, output_tokens, latency_ms, started_at, finished_at) VALUES (?, 'MODEL-RUN-1', 1, 'gateway/primary', 'completed', 12, 3, 40, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))",
    [randomUUID()]
  );
  await pool.execute(
    "INSERT INTO agent_artifacts (artifact_uuid, schema_version, task_uuid, run_uuid, artifact_type, version, title, media_type, object_bucket, object_key, content_sha256, size_bytes, metadata_json) VALUES (?, 'dipole.agent.artifact.v1', 'TASK-EVAL-1', 'RUN-EVAL-1', 'conversation_digest', 1, 'Digest', 'text/markdown', 'agent-artifacts', ?, ?, 8, JSON_OBJECT())",
    ["b".repeat(64), "v1/TASK-EVAL-1/digest.md", "c".repeat(64)]
  );
  await pool.execute(
    "INSERT INTO agent_tool_invocations (invocation_uuid, tenant_id, principal_uuid, agent_uuid, task_uuid, run_uuid, transport, tool_name, capability_id, arguments_sha256, status, result_sha256, result_bytes, latency_ms, started_at, finished_at) VALUES ('INV-EVAL-1', 'dipole', 'U1', 'AI1', 'TASK-EVAL-1', 'RUN-EVAL-1', 'mcp', 'conversation_list', 'conversation.list', ?, 'completed', ?, 2, 5, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))",
    ["d".repeat(64), "e".repeat(64)]
  );
}
