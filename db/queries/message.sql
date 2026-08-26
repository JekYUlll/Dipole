-- name: CreateMessage :execresult
INSERT INTO messages (
    uuid, client_message_id, conversation_key, seq, sender_uuid, target_type,
    target_uuid, message_type, content, file_id, file_name, file_size,
    file_url, file_content_type, file_expires_at, sent_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: EnsureConversationSequence :execresult
INSERT INTO conversation_sequences (conversation_key, last_seq)
VALUES (?, 0)
ON DUPLICATE KEY UPDATE conversation_key = VALUES(conversation_key);

-- name: LockConversationSequence :one
SELECT last_seq FROM conversation_sequences
WHERE conversation_key = ?
FOR UPDATE;

-- name: GetConversationSequence :one
SELECT last_seq FROM conversation_sequences
WHERE conversation_key = ?;

-- name: AdvanceConversationSequence :exec
UPDATE conversation_sequences
SET last_seq = ?, updated_at = NOW(3)
WHERE conversation_key = ?;

-- name: GetMessageByUUID :one
SELECT * FROM messages WHERE uuid = ? LIMIT 1;

-- name: GetMessageBySenderAndClientID :one
SELECT * FROM messages WHERE sender_uuid = ? AND client_message_id = ? LIMIT 1;

-- name: HasConversationMessages :one
SELECT EXISTS(SELECT 1 FROM messages WHERE conversation_key = ?) AS has_messages;

-- name: ListMessagesByConversationBefore :many
SELECT * FROM messages
WHERE conversation_key = sqlc.arg(conversation_key)
  AND (sqlc.arg(before_id) = 0 OR id < sqlc.arg(before_id))
ORDER BY id DESC
LIMIT ?;

-- name: ListMessagesByConversationAfter :many
SELECT * FROM messages
WHERE conversation_key = sqlc.arg(conversation_key)
  AND (sqlc.arg(after_id) = 0 OR id > sqlc.arg(after_id))
ORDER BY id ASC
LIMIT ?;

-- name: ListMessagesByConversationSeqBefore :many
SELECT * FROM messages
WHERE conversation_key = sqlc.arg(conversation_key)
  AND (sqlc.arg(before_seq) = 0 OR seq < sqlc.arg(before_seq))
ORDER BY seq DESC
LIMIT ?;

-- name: ListMessagesByConversationSeqAfter :many
SELECT * FROM messages
WHERE conversation_key = sqlc.arg(conversation_key)
  AND seq > sqlc.arg(after_seq)
ORDER BY seq ASC
LIMIT ?;

-- name: ListMessagesByUUIDs :many
SELECT * FROM messages WHERE uuid IN (sqlc.slice('uuids'));

-- name: ListOfflineMessagesByUser :many
SELECT messages.* FROM messages
WHERE messages.id > sqlc.arg(after_id)
  AND (
    (messages.target_type = sqlc.arg(direct_type) AND messages.target_uuid = sqlc.arg(user_uuid))
    OR
    (messages.target_type = sqlc.arg(group_type)
      AND messages.sender_uuid <> sqlc.arg(user_uuid)
      AND EXISTS (
        SELECT 1 FROM group_members gm
        JOIN `groups` g ON g.uuid = gm.group_uuid
        WHERE gm.group_uuid = messages.target_uuid
          AND gm.user_uuid = sqlc.arg(user_uuid)
          AND g.status IN (sqlc.arg(group_normal_status), sqlc.arg(group_dismissed_status))
      ))
  )
ORDER BY messages.id ASC
LIMIT ?;

-- name: FindLatestAccessibleFileMessage :one
SELECT messages.* FROM messages
WHERE file_id = sqlc.arg(file_uuid)
  AND message_type = sqlc.arg(file_message_type)
  AND (
    (target_type = sqlc.arg(direct_type)
      AND (sender_uuid = sqlc.arg(user_uuid) OR target_uuid = sqlc.arg(user_uuid)))
    OR
    (target_type = sqlc.arg(group_type)
      AND EXISTS (
        SELECT 1 FROM group_members gm
        JOIN `groups` g ON g.uuid = gm.group_uuid
        WHERE gm.group_uuid = messages.target_uuid
          AND gm.user_uuid = sqlc.arg(user_uuid)
          AND g.status IN (sqlc.arg(group_normal_status), sqlc.arg(group_dismissed_status))
      ))
  )
ORDER BY sent_at DESC, id DESC
LIMIT 1;
