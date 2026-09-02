import type { Pool, RowDataPacket } from "mysql2/promise";
import { z } from "zod";

import type { ShadowEvalObservation } from "./shadow-eval-adapter.js";
import {
  GET_AGENT_EVAL_OBSERVATION_HEADER,
  LIST_AGENT_EVAL_OBSERVATION_ARTIFACTS,
  LIST_AGENT_EVAL_OBSERVATION_MODEL_CALLS,
  LIST_AGENT_EVAL_OBSERVATION_STEPS,
  LIST_AGENT_EVAL_OBSERVATION_TOOL_CALLS
} from "./mysql-shadow-eval-queries.js";

interface HeaderRow extends RowDataPacket {
  task_uuid: string;
  task_status: string;
  workflow_status: string | null;
  run_uuid: string;
  run_status: string;
  trace_id: string | null;
  context_manifest_json: unknown;
}

interface StepRow extends RowDataPacket {
  step_no: number;
  capability_id: string;
  status: string;
  attempt_count: number;
  authorization_resource_type: string | null;
  authorization_resource_id: string | null;
  authorization_action: string | null;
  authorization_decision: string | null;
  output_json: unknown;
  latency_ms: number | string | null;
}

interface ArtifactRow extends RowDataPacket {
  artifact_type: string;
  version: number;
}

interface ModelCallRow extends RowDataPacket {
  route: string;
  status: string;
  input_tokens: number | null;
  output_tokens: number | null;
  latency_ms: number | null;
}

interface ToolCallRow extends RowDataPacket {
  status: string;
  latency_ms: number | string | null;
}

const contextManifestSchema = z.object({
  selected: z.array(z.object({
    id: z.string().min(1),
    provenance: z.object({ sourceType: z.string().min(1), sourceId: z.string().min(1) }).passthrough()
  }).passthrough()).max(256),
  omitted: z.array(z.unknown()).max(256)
}).passthrough();

export interface ShadowEvalObservationStore {
  load(taskId: string, runId: string): Promise<ShadowEvalObservation>;
}

export class MySQLShadowEvalObservationStore implements ShadowEvalObservationStore {
  constructor(private readonly pool: Pick<Pool, "execute">) {}

  async load(taskId: string, runId: string): Promise<ShadowEvalObservation> {
    taskId = required(taskId, "Task ID");
    runId = required(runId, "Run ID");
    const [headers] = await this.pool.execute<HeaderRow[]>(GET_AGENT_EVAL_OBSERVATION_HEADER, [runId, taskId]);
    const header = headers[0];
    if (header === undefined) throw new Error(`Shadow evaluation Task ${taskId} and Run ${runId} are missing`);

    const [steps] = await this.pool.execute<StepRow[]>(LIST_AGENT_EVAL_OBSERVATION_STEPS, [taskId]);
    const [artifacts] = await this.pool.execute<ArtifactRow[]>(LIST_AGENT_EVAL_OBSERVATION_ARTIFACTS, [taskId, runId]);
    const [modelCalls] = await this.pool.execute<ModelCallRow[]>(LIST_AGENT_EVAL_OBSERVATION_MODEL_CALLS, [taskId]);
    const [toolCalls] = await this.pool.execute<ToolCallRow[]>(LIST_AGENT_EVAL_OBSERVATION_TOOL_CALLS, [taskId, runId]);
    const contextManifest = contextManifestSchema.parse(decodedJSON(header.context_manifest_json));

    return {
      taskId: header.task_uuid, taskStatus: header.task_status, workflowStatus: header.workflow_status,
      runId: header.run_uuid, runStatus: header.run_status,
      traceId: required(header.trace_id ?? "", "Trace ID"),
      contextManifest,
      steps: steps.map(item => {
        const reason = skipReason(item);
        return {
          stepNo: item.step_no, capabilityId: item.capability_id, status: item.status, attemptCount: item.attempt_count,
          latencyMs: nullableSafeInteger(item.latency_ms, "Step latency"),
          authorization: authorization(item),
          ...(reason === undefined ? {} : { skipReason: reason })
        };
      }),
      artifacts: artifacts.map(item => ({ artifactType: item.artifact_type, version: item.version })),
      modelCalls: modelCalls.map(item => ({
        route: item.route, status: item.status, inputTokens: item.input_tokens,
        outputTokens: item.output_tokens, latencyMs: item.latency_ms
      })),
      toolCalls: toolCalls.map(item => ({ status: item.status, latencyMs: nullableSafeInteger(item.latency_ms, "Tool latency") }))
    };
  }
}

function skipReason(row: StepRow): "no_discovered_conversation" | undefined {
  if (row.capability_id !== "conversation.read" || row.status !== "completed") return undefined;
  const decoded = decodedJSON(row.output_json);
  if (decoded === null || typeof decoded !== "object" || Array.isArray(decoded)) return undefined;
  const output = decoded as { readonly status?: unknown; readonly reason?: unknown };
  if (output.status === "skipped" && output.reason === "no_discovered_conversation" && Object.keys(output).length === 2) {
    return "no_discovered_conversation";
  }
  return undefined;
}

function authorization(row: StepRow) {
  const values = [row.authorization_resource_type, row.authorization_resource_id, row.authorization_action, row.authorization_decision];
  if (values.every(value => value === null)) return null;
  if (values.some(value => value === null)) throw new Error("Shadow evaluation Step authorization is incomplete");
  if (values.some(value => value!.trim() === "") || row.authorization_decision !== "allowed") throw new Error("Shadow evaluation Step authorization is invalid");
  return { resourceType: row.authorization_resource_type!, resourceId: row.authorization_resource_id!, action: row.authorization_action!, decision: "allowed" as const };
}

function decodedJSON(value: unknown): unknown {
  return typeof value === "string" ? JSON.parse(value) as unknown : value;
}

function required(value: string, label: string): string {
  value = value.trim();
  if (value.length < 1 || value.length > 64) throw new Error(`Shadow evaluation ${label} must contain 1-64 characters`);
  return value;
}

function nullableSafeInteger(value: number | string | null, label: string): number | null {
  if (value === null) return null;
  const number = typeof value === "string" ? Number(value) : value;
  if (!Number.isSafeInteger(number) || number < 0) throw new Error(`Shadow evaluation ${label} is invalid`);
  return number;
}
