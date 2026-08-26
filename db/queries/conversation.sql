-- name: UpsertConversationMessage :execresult
INSERT INTO conversations (
    user_uuid,
    target_type,
    target_uuid,
    conversation_key,
    last_message_uuid,
    last_message_type,
    last_message_preview,
    last_message_at,
    last_message_sender_uuid,
    unread_count,
    remark,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, sqlc.arg(unread_increment), '', NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
    unread_count = CASE
        WHEN last_message_uuid <> VALUES(last_message_uuid)
        THEN CASE
            WHEN sqlc.arg(unread_increment) > 0
            THEN unread_count + sqlc.arg(unread_increment)
            ELSE 0
        END
        ELSE unread_count
    END,
    target_type = VALUES(target_type),
    target_uuid = VALUES(target_uuid),
    last_message_uuid = VALUES(last_message_uuid),
    last_message_type = VALUES(last_message_type),
    last_message_preview = VALUES(last_message_preview),
    last_message_at = VALUES(last_message_at),
    last_message_sender_uuid = VALUES(last_message_sender_uuid),
    updated_at = NOW(3);

-- name: ListConversationsByUser :many
SELECT id, user_uuid, target_type, target_uuid, conversation_key,
       last_message_uuid, last_message_type, last_message_preview,
       last_message_at, last_message_sender_uuid, unread_count, remark,
       created_at, updated_at
FROM conversations
WHERE user_uuid = ?
ORDER BY last_message_at DESC
LIMIT ?;

-- name: GetConversationByUserAndKey :one
SELECT id, user_uuid, target_type, target_uuid, conversation_key,
       last_message_uuid, last_message_type, last_message_preview,
       last_message_at, last_message_sender_uuid, unread_count, remark,
       created_at, updated_at
FROM conversations
WHERE user_uuid = ? AND conversation_key = ?
LIMIT 1;

-- name: InitGroupConversation :execresult
INSERT INTO conversations (
    user_uuid,
    target_type,
    target_uuid,
    conversation_key,
    last_message_uuid,
    last_message_type,
    last_message_preview,
    last_message_at,
    last_message_sender_uuid,
    unread_count,
    remark,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, '', 0, '', ?, '', 0, '', NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE id = id;

-- name: UpdateConversationRemark :execresult
UPDATE conversations
SET remark = ?,
    updated_at = NOW(3)
WHERE user_uuid = ? AND conversation_key = ?;

-- name: ClearConversationUnread :execresult
UPDATE conversations
SET unread_count = 0,
    updated_at = NOW(3)
WHERE user_uuid = ? AND conversation_key = ?;
