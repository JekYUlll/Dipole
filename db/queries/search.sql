-- name: ApplyMessageSearchState :exec
INSERT INTO message_search_documents (
    message_uuid, conversation_key, message_seq, revision, sender_uuid, message_type,
    content, sent_at, searchable, payload_hash, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
    conversation_key = IF(VALUES(revision) > revision, VALUES(conversation_key), conversation_key),
    message_seq = IF(VALUES(revision) > revision, VALUES(message_seq), message_seq),
    sender_uuid = IF(VALUES(revision) > revision, VALUES(sender_uuid), sender_uuid),
    message_type = IF(VALUES(revision) > revision, VALUES(message_type), message_type),
    content = IF(VALUES(revision) > revision, VALUES(content), content),
    sent_at = IF(VALUES(revision) > revision, VALUES(sent_at), sent_at),
    searchable = IF(VALUES(revision) > revision, VALUES(searchable), searchable),
    payload_hash = IF(VALUES(revision) > revision, VALUES(payload_hash), payload_hash),
    updated_at = IF(VALUES(revision) > revision, NOW(3), updated_at),
    revision = GREATEST(revision, VALUES(revision));

-- name: GetMessageSearchState :one
SELECT message_uuid, conversation_key, message_seq, revision, sender_uuid, message_type,
       content, sent_at, searchable, payload_hash
FROM message_search_documents
WHERE message_uuid = ?
LIMIT 1;

-- name: SearchMessageDocuments :many
SELECT message_uuid, conversation_key, message_seq, revision, sender_uuid, message_type, content, sent_at
FROM message_search_documents
WHERE conversation_key IN (sqlc.slice('conversation_keys'))
  AND searchable = TRUE
  AND LOWER(content) LIKE CONCAT('%', LOWER(sqlc.arg(search_text)), '%') ESCAPE '\\'
ORDER BY sent_at DESC, message_uuid DESC
LIMIT ?;
