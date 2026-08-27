-- name: InsertAgentShadowPlan :execrows
INSERT INTO agent_shadow_plans (
    task_uuid, event_id, event_type, summary, plan_sha256, model_route,
    model_attempts, model_input_tokens, model_output_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAgentShadowPlan :one
SELECT task_uuid, event_id, event_type, plan_sha256
FROM agent_shadow_plans
WHERE task_uuid = ?
LIMIT 1;

-- name: InsertAgentShadowStep :exec
INSERT INTO agent_shadow_steps (
    task_uuid, step_no, capability_id, status, input_json
) VALUES (?, ?, ?, 'planned', ?);

-- name: ProbeAgentShadowPlans :many
SELECT task_uuid FROM agent_shadow_plans LIMIT 1;
