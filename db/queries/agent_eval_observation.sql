-- name: GetAgentEvalObservationHeader :one
SELECT
    t.task_uuid,
    t.status AS task_status,
    r.run_uuid,
    r.status AS run_status,
    p.context_manifest_json
FROM agent_tasks AS t
JOIN agent_runs AS r ON r.task_uuid = t.task_uuid AND r.run_uuid = ? AND r.mode = 'shadow'
JOIN agent_shadow_plans AS p ON p.task_uuid = t.task_uuid
WHERE t.task_uuid = ? AND p.context_manifest_json IS NOT NULL
LIMIT 1;

-- name: ListAgentEvalObservationSteps :many
SELECT step_no, capability_id, status, attempt_count,
       TIMESTAMPDIFF(MICROSECOND, started_at, finished_at) DIV 1000 AS latency_ms
FROM agent_shadow_steps
WHERE task_uuid = ?
ORDER BY step_no ASC
LIMIT 257;

-- name: ListAgentEvalObservationArtifacts :many
SELECT artifact_type, version
FROM agent_artifacts
WHERE task_uuid = ? AND run_uuid = ?
ORDER BY artifact_type ASC, version ASC
LIMIT 257;

-- name: ListAgentEvalObservationModelCalls :many
SELECT c.route, c.status, c.input_tokens, c.output_tokens, c.latency_ms
FROM agent_model_runs AS r
JOIN agent_model_calls AS c ON c.run_uuid = r.run_uuid
WHERE r.task_uuid = ?
ORDER BY c.call_no ASC
LIMIT 65;

-- name: ListAgentEvalObservationToolCalls :many
SELECT status, latency_ms
FROM agent_tool_invocations
WHERE task_uuid = ? AND run_uuid = ?
ORDER BY started_at ASC, invocation_uuid ASC
LIMIT 257;
