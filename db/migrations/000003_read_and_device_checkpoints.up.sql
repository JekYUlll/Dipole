ALTER TABLE conversations
    ADD COLUMN last_message_seq BIGINT UNSIGNED NULL AFTER last_message_uuid,
    ADD COLUMN read_seq BIGINT UNSIGNED NULL AFTER last_message_seq;

UPDATE conversations AS conversation
LEFT JOIN messages AS message ON message.uuid = conversation.last_message_uuid
SET conversation.last_message_seq = COALESCE(message.seq, 0),
    conversation.read_seq = CASE
        WHEN message.seq IS NULL OR conversation.unread_count >= message.seq THEN 0
        ELSE message.seq - conversation.unread_count
    END;

ALTER TABLE conversations
    MODIFY COLUMN last_message_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    MODIFY COLUMN read_seq BIGINT UNSIGNED NOT NULL DEFAULT 0;

CREATE TABLE device_sync_checkpoints (
    user_uuid VARCHAR(24) NOT NULL,
    device_id VARCHAR(128) NOT NULL,
    sync_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_uuid, device_id),
    KEY idx_device_sync_checkpoints_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
