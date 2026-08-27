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
