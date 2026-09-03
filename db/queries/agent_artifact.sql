-- name: InsertAgentArtifact :execrows
INSERT IGNORE INTO agent_artifacts (
    artifact_uuid, schema_version, task_uuid, run_uuid, artifact_type, version,
    title, media_type, object_bucket, object_key, content_sha256, size_bytes,
    metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAgentArtifact :one
SELECT * FROM agent_artifacts WHERE artifact_uuid = ? LIMIT 1;

-- name: GetAgentArtifactByTaskTypeVersion :one
SELECT * FROM agent_artifacts
WHERE task_uuid = ? AND artifact_type = ? AND version = ?
LIMIT 1;

-- name: ListOwnedAgentArtifactMetadata :many
SELECT a.*
FROM agent_artifacts AS a
JOIN agent_tasks AS t ON t.task_uuid = a.task_uuid
WHERE t.tenant_id = sqlc.arg(tenant_id)
  AND t.principal_uuid = sqlc.arg(principal_uuid)
  AND (a.created_at < sqlc.arg(after_created_at)
       OR (a.created_at = sqlc.arg(after_created_at) AND a.artifact_uuid < sqlc.arg(after_artifact_uuid)))
ORDER BY a.created_at DESC, a.artifact_uuid DESC
LIMIT ?;

-- name: AgentArtifactExistsByObjectKey :one
SELECT EXISTS(
    SELECT 1 FROM agent_artifacts
    WHERE object_bucket = ? AND object_key = ?
);
