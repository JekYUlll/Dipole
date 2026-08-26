-- name: UpsertMessageSearchDocument :exec
INSERT INTO message_search_documents (
    message_uuid, conversation_key, message_seq, sender_uuid, message_type,
    content, sent_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
    conversation_key = VALUES(conversation_key),
    message_seq = VALUES(message_seq),
    sender_uuid = VALUES(sender_uuid),
    message_type = VALUES(message_type),
    content = VALUES(content),
    sent_at = VALUES(sent_at),
    updated_at = NOW(3);

-- name: DeleteMessageSearchDocument :exec
DELETE FROM message_search_documents WHERE message_uuid = ?;

-- name: SearchMessageDocuments :many
SELECT message_uuid, conversation_key, message_seq, sender_uuid, message_type, content, sent_at
FROM message_search_documents
WHERE conversation_key IN (sqlc.slice('conversation_keys'))
  AND LOWER(content) LIKE CONCAT('%', LOWER(sqlc.arg(search_text)), '%') ESCAPE '\\'
ORDER BY sent_at DESC, message_uuid DESC
LIMIT ?;
