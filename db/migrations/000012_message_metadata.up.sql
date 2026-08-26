CREATE TABLE message_metadata (
    message_uuid VARCHAR(24) NOT NULL,
    client_message_id VARCHAR(64) NOT NULL,
    conversation_key VARCHAR(64) NOT NULL,
    message_seq BIGINT UNSIGNED NOT NULL,
    sender_uuid VARCHAR(24) NOT NULL,
    target_type TINYINT NOT NULL,
    target_uuid VARCHAR(24) NOT NULL,
    message_type TINYINT NOT NULL,
    file_id VARCHAR(24) NOT NULL DEFAULT '',
    file_expires_at DATETIME(3) NULL,
    payload_sha256 CHAR(64) NOT NULL DEFAULT '',
    sent_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (message_uuid),
    UNIQUE KEY idx_message_metadata_sender_client (sender_uuid, client_message_id),
    UNIQUE KEY idx_message_metadata_conversation_seq (conversation_key, message_seq),
    KEY idx_message_metadata_file_sent (file_id, sent_at),
    KEY idx_message_metadata_target (target_type, target_uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO message_metadata (
    message_uuid, client_message_id, conversation_key, message_seq, sender_uuid,
    target_type, target_uuid, message_type, file_id, file_expires_at,
    payload_sha256, sent_at, created_at, updated_at
)
SELECT
    uuid, client_message_id, conversation_key, seq, sender_uuid,
    target_type, target_uuid, message_type, file_id, file_expires_at,
    '', sent_at, COALESCE(created_at, NOW(3)), COALESCE(updated_at, NOW(3))
FROM messages;
