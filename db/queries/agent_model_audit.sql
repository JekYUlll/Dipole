-- name: InsertAgentModelRun :exec
INSERT IGNORE INTO agent_model_runs (
    run_uuid, task_uuid, status, max_calls, total_timeout_ms, max_output_tokens_per_call, started_at
) VALUES (?, ?, 'running', ?, ?, ?, UTC_TIMESTAMP());

-- name: LockAgentModelRun :one
SELECT run_uuid, task_uuid, status, max_calls, total_timeout_ms, max_output_tokens_per_call, calls_reserved
FROM agent_model_runs
WHERE task_uuid = ?
FOR UPDATE;

-- name: IncrementAgentModelRunCalls :execrows
UPDATE agent_model_runs
SET calls_reserved = calls_reserved + 1
WHERE run_uuid = ? AND status = 'running' AND calls_reserved < max_calls;

-- name: InsertAgentModelCall :exec
INSERT INTO agent_model_calls (
    call_uuid, run_uuid, call_no, route, status, started_at
) VALUES (?, ?, ?, ?, 'reserved', UTC_TIMESTAMP());

-- name: CompleteAgentModelCall :execrows
UPDATE agent_model_calls
SET status = 'completed', output_json = ?, input_tokens = ?, output_tokens = ?, finish_reason = ?, latency_ms = ?, finished_at = UTC_TIMESTAMP(), last_error = NULL
WHERE call_uuid = ? AND run_uuid = ? AND call_no = ? AND status = 'reserved';

-- name: GetCompletedAgentModelCall :one
SELECT c.run_uuid, c.call_uuid, c.call_no, c.route, c.output_json, c.input_tokens, c.output_tokens,
       r.max_calls, r.total_timeout_ms, r.max_output_tokens_per_call
FROM agent_model_calls c
JOIN agent_model_runs r ON r.run_uuid = c.run_uuid
WHERE r.task_uuid = ? AND c.status = 'completed'
ORDER BY c.call_no DESC
LIMIT 1;

-- name: GetAgentModelRunStatus :one
SELECT status
FROM agent_model_runs
WHERE run_uuid = ?
LIMIT 1;

-- name: FailAgentModelCall :execrows
UPDATE agent_model_calls
SET status = 'failed', latency_ms = ?, finished_at = UTC_TIMESTAMP(), last_error = ?
WHERE call_uuid = ? AND run_uuid = ? AND call_no = ? AND status = 'reserved';

-- name: AbandonAgentModelCalls :execrows
UPDATE agent_model_calls
SET status = 'abandoned', finished_at = UTC_TIMESTAMP(), last_error = ?
WHERE run_uuid = ? AND status = 'reserved';

-- name: CompleteAgentModelRun :execrows
UPDATE agent_model_runs
SET status = 'completed', completed_at = UTC_TIMESTAMP(), last_error = NULL
WHERE run_uuid = ? AND status = 'running';

-- name: FailAgentModelRun :execrows
UPDATE agent_model_runs
SET status = 'failed', completed_at = UTC_TIMESTAMP(), last_error = ?
WHERE run_uuid = ? AND status = 'running';

-- name: FailAgentModelRunByTask :execrows
UPDATE agent_model_runs
SET status = 'failed', completed_at = UTC_TIMESTAMP(), last_error = ?
WHERE task_uuid = ? AND status = 'running';

-- name: ProbeAgentModelRuns :many
SELECT run_uuid FROM agent_model_runs LIMIT 1;
