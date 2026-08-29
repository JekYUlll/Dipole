// Code generated from db/queries/agent_model_audit.sql; DO NOT EDIT.

export const INSERT_AGENT_MODEL_RUN = "INSERT IGNORE INTO agent_model_runs (\n    run_uuid, task_uuid, status, max_calls, total_timeout_ms, max_output_tokens_per_call, started_at\n) VALUES (?, ?, 'running', ?, ?, ?, UTC_TIMESTAMP())";

export const LOCK_AGENT_MODEL_RUN = "SELECT run_uuid, task_uuid, status, max_calls, total_timeout_ms, max_output_tokens_per_call, calls_reserved\nFROM agent_model_runs\nWHERE task_uuid = ?\nFOR UPDATE";

export const INCREMENT_AGENT_MODEL_RUN_CALLS = "UPDATE agent_model_runs\nSET calls_reserved = calls_reserved + 1\nWHERE run_uuid = ? AND status = 'running' AND calls_reserved < max_calls";

export const INSERT_AGENT_MODEL_CALL = "INSERT INTO agent_model_calls (\n    call_uuid, run_uuid, call_no, route, status, started_at\n) VALUES (?, ?, ?, ?, 'reserved', UTC_TIMESTAMP())";

export const COMPLETE_AGENT_MODEL_CALL = "UPDATE agent_model_calls\nSET status = 'completed', output_json = ?, input_tokens = ?, output_tokens = ?, finish_reason = ?, latency_ms = ?, finished_at = UTC_TIMESTAMP(), last_error = NULL\nWHERE call_uuid = ? AND run_uuid = ? AND call_no = ? AND status = 'reserved'";

export const GET_COMPLETED_AGENT_MODEL_CALL = "SELECT c.run_uuid, c.call_uuid, c.call_no, c.route, c.output_json, c.input_tokens, c.output_tokens,\n       r.max_calls, r.total_timeout_ms, r.max_output_tokens_per_call\nFROM agent_model_calls c\nJOIN agent_model_runs r ON r.run_uuid = c.run_uuid\nWHERE r.task_uuid = ? AND c.status = 'completed'\nORDER BY c.call_no DESC\nLIMIT 1";

export const GET_AGENT_MODEL_RUN_STATUS = "SELECT status\nFROM agent_model_runs\nWHERE run_uuid = ?\nLIMIT 1";

export const FAIL_AGENT_MODEL_CALL = "UPDATE agent_model_calls\nSET status = 'failed', latency_ms = ?, finished_at = UTC_TIMESTAMP(), last_error = ?\nWHERE call_uuid = ? AND run_uuid = ? AND call_no = ? AND status = 'reserved'";

export const ABANDON_AGENT_MODEL_CALLS = "UPDATE agent_model_calls\nSET status = 'abandoned', finished_at = UTC_TIMESTAMP(), last_error = ?\nWHERE run_uuid = ? AND status = 'reserved'";

export const COMPLETE_AGENT_MODEL_RUN = "UPDATE agent_model_runs\nSET status = 'completed', completed_at = UTC_TIMESTAMP(), last_error = NULL\nWHERE run_uuid = ? AND status = 'running'";

export const FAIL_AGENT_MODEL_RUN = "UPDATE agent_model_runs\nSET status = 'failed', completed_at = UTC_TIMESTAMP(), last_error = ?\nWHERE run_uuid = ? AND status = 'running'";

export const FAIL_AGENT_MODEL_RUN_BY_TASK = "UPDATE agent_model_runs\nSET status = 'failed', completed_at = UTC_TIMESTAMP(), last_error = ?\nWHERE task_uuid = ? AND status = 'running'";

export const PROBE_AGENT_MODEL_RUNS = "SELECT run_uuid FROM agent_model_runs LIMIT 1";
