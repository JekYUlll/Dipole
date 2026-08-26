-- name: CreateMessageMetadata :exec
INSERT INTO message_metadata (
    message_uuid, client_message_id, conversation_key, message_seq, sender_uuid,
    target_type, target_uuid, message_type, file_id, file_expires_at,
    payload_sha256, sent_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetMessageMetadataByUUID :one
SELECT * FROM message_metadata WHERE message_uuid = ? LIMIT 1;

-- name: GetMessageMetadataBySenderAndClientID :one
SELECT * FROM message_metadata
WHERE sender_uuid = ? AND client_message_id = ?
LIMIT 1;

-- name: FindLatestAccessibleFileMetadata :one
SELECT metadata.* FROM message_metadata AS metadata
WHERE metadata.file_id = sqlc.arg(file_uuid)
  AND metadata.message_type = sqlc.arg(file_message_type)
  AND (
    (metadata.target_type = sqlc.arg(direct_type)
      AND (metadata.sender_uuid = sqlc.arg(user_uuid) OR metadata.target_uuid = sqlc.arg(user_uuid)))
    OR
    (metadata.target_type = sqlc.arg(group_type)
      AND EXISTS (
        SELECT 1 FROM group_members gm
        JOIN `groups` g ON g.uuid = gm.group_uuid
        WHERE gm.group_uuid = metadata.target_uuid
          AND gm.user_uuid = sqlc.arg(user_uuid)
          AND g.status IN (sqlc.arg(group_normal_status), sqlc.arg(group_dismissed_status))
      ))
  )
ORDER BY metadata.sent_at DESC, metadata.message_uuid DESC
LIMIT 1;
