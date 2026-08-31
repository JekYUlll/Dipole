// Code generated from db/queries/agent_eval_observation.sql; DO NOT EDIT.

export const GET_AGENT_EVAL_OBSERVATION_HEADER = "SELECT\n    t.task_uuid,\n    t.status AS task_status,\n    t.workflow_status,\n    r.run_uuid,\n    r.status AS run_status,\n    r.trace_id,\n    p.context_manifest_json\nFROM agent_tasks AS t\nJOIN agent_runs AS r ON r.task_uuid = t.task_uuid AND r.run_uuid = ? AND r.mode = 'shadow'\nJOIN agent_shadow_plans AS p ON p.task_uuid = t.task_uuid\nWHERE t.task_uuid = ? AND p.context_manifest_json IS NOT NULL\nLIMIT 1";

export const LIST_AGENT_EVAL_OBSERVATION_STEPS = "SELECT step_no, capability_id, status, attempt_count,\n       TIMESTAMPDIFF(MICROSECOND, started_at, finished_at) DIV 1000 AS latency_ms\nFROM agent_shadow_steps\nWHERE task_uuid = ?\nORDER BY step_no ASC\nLIMIT 257";

export const LIST_AGENT_EVAL_OBSERVATION_ARTIFACTS = "SELECT artifact_type, version\nFROM agent_artifacts\nWHERE task_uuid = ? AND run_uuid = ?\nORDER BY artifact_type ASC, version ASC\nLIMIT 257";

export const LIST_AGENT_EVAL_OBSERVATION_MODEL_CALLS = "SELECT c.route, c.status, c.input_tokens, c.output_tokens, c.latency_ms\nFROM agent_model_runs AS r\nJOIN agent_model_calls AS c ON c.run_uuid = r.run_uuid\nWHERE r.task_uuid = ?\nORDER BY c.call_no ASC\nLIMIT 65";

export const LIST_AGENT_EVAL_OBSERVATION_TOOL_CALLS = "SELECT status, latency_ms\nFROM agent_tool_invocations\nWHERE task_uuid = ? AND run_uuid = ?\nORDER BY started_at ASC, invocation_uuid ASC\nLIMIT 257";
