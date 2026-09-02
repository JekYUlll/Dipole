-- name: InsertAgentContextAblationBinding :exec
INSERT INTO agent_context_ablation_bindings (
    experiment_uuid, case_sha256, condition_name, task_uuid, run_uuid, candidate_version
) VALUES (?, ?, ?, ?, ?, ?);

-- name: ListAgentContextAblationBindings :many
SELECT experiment_uuid, case_sha256, condition_name, task_uuid, run_uuid, candidate_version, created_at
FROM agent_context_ablation_bindings
WHERE experiment_uuid = ?
ORDER BY case_sha256 ASC, condition_name ASC;
