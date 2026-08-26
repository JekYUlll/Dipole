-- name: InsertAICallLog :execrows
INSERT IGNORE INTO ai_call_logs (
    trigger_message_uuid,
    response_message_uuid,
    conversation_key,
    user_uuid,
    assistant_uuid,
    provider,
    model,
    status,
    error_message,
    prompt_tokens,
    completion_tokens,
    total_tokens,
    latency_ms,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3));

-- name: MarkAICallLogSucceeded :exec
UPDATE ai_call_logs
SET status = ?,
    response_message_uuid = ?,
    prompt_tokens = ?,
    completion_tokens = ?,
    total_tokens = ?,
    latency_ms = ?,
    error_message = '',
    updated_at = NOW(3)
WHERE trigger_message_uuid = ?;

-- name: MarkAICallLogFailed :exec
UPDATE ai_call_logs
SET status = ?,
    error_message = ?,
    latency_ms = ?,
    updated_at = NOW(3)
WHERE trigger_message_uuid = ?;
