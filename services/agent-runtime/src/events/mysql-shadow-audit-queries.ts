// Code generated from db/queries/agent_shadow_trajectory.sql; DO NOT EDIT.

export const INSERT_AGENT_SHADOW_PLAN = "INSERT INTO agent_shadow_plans (\n    task_uuid, event_id, event_type, summary, plan_sha256, model_route,\n    model_attempts, model_input_tokens, model_output_tokens, context_compiler_version,\n    context_estimated_tokens, context_manifest_json\n) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)";

export const GET_AGENT_SHADOW_PLAN = "SELECT task_uuid, event_id, event_type, plan_sha256\nFROM agent_shadow_plans\nWHERE task_uuid = ?\nLIMIT 1";

export const INSERT_AGENT_SHADOW_STEP = "INSERT INTO agent_shadow_steps (\n    task_uuid, step_no, capability_id, status, input_json\n) VALUES (?, ?, ?, 'planned', ?)";

export const INSERT_AGENT_MEMORY_TASK_LINEAGE = "INSERT INTO agent_memory_task_lineage (\n    memory_uuid, task_uuid, representation, source\n) VALUES (?, ?, ?, ?)\nON DUPLICATE KEY UPDATE\n    representation = IF(representation = VALUES(representation), representation, NULL),\n    source = IF(VALUES(source) = 'context_pre_model', 'context_pre_model', source)";

export const CLAIM_AGENT_SHADOW_STEP = "UPDATE agent_shadow_steps\nSET status = 'running', claim_token = ?, attempt_count = attempt_count + 1,\n    started_at = UTC_TIMESTAMP(), lease_expires_at = TIMESTAMPADD(MICROSECOND, ?, UTC_TIMESTAMP()),\n    finished_at = NULL, last_error = NULL\nWHERE task_uuid = ? AND step_no = ? AND (\n    status IN ('planned', 'failed') OR\n    (status = 'running' AND lease_expires_at < UTC_TIMESTAMP())\n)";

export const GET_AGENT_SHADOW_STEP = "SELECT status, claim_token FROM agent_shadow_steps WHERE task_uuid = ? AND step_no = ? LIMIT 1";

export const COMPLETE_AGENT_SHADOW_STEP = "UPDATE agent_shadow_steps\nSET status = 'completed', output_json = ?, finished_at = UTC_TIMESTAMP(), lease_expires_at = UTC_TIMESTAMP(), last_error = NULL\nWHERE task_uuid = ? AND step_no = ? AND claim_token = ? AND status = 'running' AND lease_expires_at >= UTC_TIMESTAMP()";

export const FAIL_AGENT_SHADOW_STEP = "UPDATE agent_shadow_steps\nSET status = 'failed', finished_at = UTC_TIMESTAMP(), lease_expires_at = UTC_TIMESTAMP(), last_error = ?\nWHERE task_uuid = ? AND step_no = ? AND claim_token = ? AND status = 'running'";

export const PROBE_AGENT_SHADOW_PLANS = "SELECT task_uuid FROM agent_shadow_plans LIMIT 1";
