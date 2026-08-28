-- name: InsertAgentShadowPlan :execrows
INSERT INTO agent_shadow_plans (
    task_uuid, event_id, event_type, summary, plan_sha256, model_route,
    model_attempts, model_input_tokens, model_output_tokens, context_compiler_version,
    context_estimated_tokens, context_manifest_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAgentShadowPlan :one
SELECT task_uuid, event_id, event_type, plan_sha256
FROM agent_shadow_plans
WHERE task_uuid = ?
LIMIT 1;

-- name: InsertAgentShadowStep :exec
INSERT INTO agent_shadow_steps (
    task_uuid, step_no, capability_id, status, input_json
) VALUES (?, ?, ?, 'planned', ?);

-- name: ClaimAgentShadowStep :execrows
UPDATE agent_shadow_steps
SET status = 'running', claim_token = ?, attempt_count = attempt_count + 1,
    started_at = UTC_TIMESTAMP(), lease_expires_at = TIMESTAMPADD(MICROSECOND, ?, UTC_TIMESTAMP()),
    finished_at = NULL, last_error = NULL
WHERE task_uuid = ? AND step_no = ? AND (
    status IN ('planned', 'failed') OR
    (status = 'running' AND lease_expires_at < UTC_TIMESTAMP())
);

-- name: GetAgentShadowStep :one
SELECT status, claim_token FROM agent_shadow_steps WHERE task_uuid = ? AND step_no = ? LIMIT 1;

-- name: CompleteAgentShadowStep :execrows
UPDATE agent_shadow_steps
SET status = 'completed', output_json = ?, finished_at = UTC_TIMESTAMP(), lease_expires_at = UTC_TIMESTAMP(), last_error = NULL
WHERE task_uuid = ? AND step_no = ? AND claim_token = ? AND status = 'running' AND lease_expires_at >= UTC_TIMESTAMP();

-- name: FailAgentShadowStep :execrows
UPDATE agent_shadow_steps
SET status = 'failed', finished_at = UTC_TIMESTAMP(), lease_expires_at = UTC_TIMESTAMP(), last_error = ?
WHERE task_uuid = ? AND step_no = ? AND claim_token = ? AND status = 'running';

-- name: ProbeAgentShadowPlans :many
SELECT task_uuid FROM agent_shadow_plans LIMIT 1;
