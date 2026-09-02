-- name: UpsertConversationMessage :execresult
INSERT INTO conversations (
    user_uuid,
    target_type,
    target_uuid,
    conversation_key,
    last_message_uuid,
    last_message_seq,
    read_seq,
    last_message_type,
    last_message_preview,
    last_message_at,
    last_message_sender_uuid,
    unread_count,
    remark,
    created_at,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, sqlc.arg(last_message_seq),
    sqlc.arg(initial_read_seq),
    ?, ?, ?, ?, CAST(sqlc.arg(unread_increment) AS SIGNED), '', NOW(3), NOW(3)
)
ON DUPLICATE KEY UPDATE
    unread_count = CASE
        WHEN VALUES(last_message_seq) > last_message_seq AND CAST(sqlc.arg(unread_increment) AS SIGNED) > 0
            THEN VALUES(last_message_seq) - read_seq
        WHEN VALUES(last_message_seq) > last_message_seq THEN 0
        ELSE unread_count
    END,
    read_seq = CASE
        WHEN VALUES(last_message_seq) > last_message_seq AND CAST(sqlc.arg(unread_increment) AS SIGNED) = 0
            THEN VALUES(last_message_seq)
        ELSE read_seq
    END,
    target_type = VALUES(target_type),
    target_uuid = VALUES(target_uuid),
    last_message_uuid = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_uuid), last_message_uuid),
    last_message_type = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_type), last_message_type),
    last_message_preview = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_preview), last_message_preview),
    last_message_at = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_at), last_message_at),
    last_message_sender_uuid = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_sender_uuid), last_message_sender_uuid),
    last_message_seq = GREATEST(last_message_seq, VALUES(last_message_seq)),
    updated_at = NOW(3);

-- name: ListConversationsByUser :many
SELECT * FROM conversations
WHERE user_uuid = ?
ORDER BY last_message_at DESC
LIMIT ?;

-- name: UpsertGroupConversationMessageBatch :execresult
INSERT INTO conversations (
    user_uuid,
    target_type,
    target_uuid,
    conversation_key,
    last_message_uuid,
    last_message_seq,
    read_seq,
    last_message_type,
    last_message_preview,
    last_message_at,
    last_message_sender_uuid,
    unread_count,
    remark,
    created_at,
    updated_at
)
SELECT
    gm.user_uuid,
    sqlc.arg(target_type) AS target_type,
    sqlc.arg(group_uuid) AS target_uuid,
    sqlc.arg(conversation_key) AS conversation_key,
    sqlc.arg(last_message_uuid) AS last_message_uuid,
    sqlc.arg(last_message_seq) AS last_message_seq,
    CASE WHEN gm.user_uuid = sqlc.arg(sender_uuid) THEN sqlc.arg(last_message_seq)
         ELSE sqlc.arg(initial_read_seq) END AS read_seq,
    sqlc.arg(last_message_type) AS last_message_type,
    sqlc.arg(last_message_preview) AS last_message_preview,
    sqlc.arg(last_message_at) AS last_message_at,
    sqlc.arg(last_message_sender_uuid) AS last_message_sender_uuid,
    CASE WHEN gm.user_uuid = sqlc.arg(sender_uuid) THEN 0 ELSE 1 END AS unread_count,
    '' AS remark, NOW(3) AS created_at, NOW(3) AS updated_at
FROM group_members gm
WHERE gm.group_uuid = sqlc.arg(group_uuid)
ON DUPLICATE KEY UPDATE
    unread_count = CASE
        WHEN VALUES(last_message_seq) > last_message_seq AND VALUES(unread_count) > 0
            THEN VALUES(last_message_seq) - read_seq
        WHEN VALUES(last_message_seq) > last_message_seq THEN 0
        ELSE unread_count
    END,
    read_seq = CASE
        WHEN VALUES(last_message_seq) > last_message_seq AND VALUES(unread_count) = 0
            THEN VALUES(last_message_seq)
        ELSE read_seq
    END,
    target_type = VALUES(target_type),
    target_uuid = VALUES(target_uuid),
    last_message_uuid = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_uuid), last_message_uuid),
    last_message_type = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_type), last_message_type),
    last_message_preview = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_preview), last_message_preview),
    last_message_at = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_at), last_message_at),
    last_message_sender_uuid = IF(VALUES(last_message_seq) > last_message_seq, VALUES(last_message_sender_uuid), last_message_sender_uuid),
    last_message_seq = GREATEST(last_message_seq, VALUES(last_message_seq)),
    updated_at = NOW(3);

-- name: ListSearchConversationKeysByUser :many
SELECT c.conversation_key FROM conversations c
WHERE c.user_uuid = sqlc.arg(user_uuid)
  AND c.target_type = sqlc.arg(direct_target_type)
UNION
SELECT CONCAT('group:', gm.group_uuid) AS conversation_key
FROM group_members gm
JOIN `groups` g ON g.uuid = gm.group_uuid
WHERE gm.user_uuid = sqlc.arg(user_uuid)
  AND g.status IN (sqlc.arg(group_normal_status), sqlc.arg(group_dismissed_status))
ORDER BY conversation_key ASC;

-- name: GetConversationByUserAndKey :one
SELECT * FROM conversations
WHERE user_uuid = ? AND conversation_key = ?
LIMIT 1;

-- name: InitGroupConversation :execresult
INSERT INTO conversations (
    user_uuid,
    target_type,
    target_uuid,
    conversation_key,
    last_message_uuid,
    last_message_seq,
    read_seq,
    last_message_type,
    last_message_preview,
    last_message_at,
    last_message_sender_uuid,
    unread_count,
    remark,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, '', 0, 0, 0, '', ?, '', 0, '', NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE id = id;

-- name: UpdateConversationRemark :execresult
UPDATE conversations
SET remark = ?,
    updated_at = NOW(3)
WHERE user_uuid = ? AND conversation_key = ?;

-- name: MarkConversationReadThrough :execresult
UPDATE conversations
SET read_seq = GREATEST(read_seq, LEAST(last_message_seq, CAST(sqlc.arg(read_through_seq) AS UNSIGNED))),
    unread_count = GREATEST(last_message_seq - read_seq, 0),
    updated_at = NOW(3)
WHERE user_uuid = ? AND conversation_key = ?;
