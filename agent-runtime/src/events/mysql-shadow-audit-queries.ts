// Code generated from db/queries/agent_shadow_trajectory.sql; DO NOT EDIT.

export const INSERT_AGENT_SHADOW_PLAN = "INSERT INTO agent_shadow_plans (\n    task_uuid, event_id, event_type, summary, plan_sha256, model_route,\n    model_attempts, model_input_tokens, model_output_tokens\n) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)";

export const GET_AGENT_SHADOW_PLAN = "SELECT task_uuid, event_id, event_type, plan_sha256\nFROM agent_shadow_plans\nWHERE task_uuid = ?\nLIMIT 1";

export const INSERT_AGENT_SHADOW_STEP = "INSERT INTO agent_shadow_steps (\n    task_uuid, step_no, capability_id, status, input_json\n) VALUES (?, ?, ?, 'planned', ?)";

export const PROBE_AGENT_SHADOW_PLANS = "SELECT task_uuid FROM agent_shadow_plans LIMIT 1";
