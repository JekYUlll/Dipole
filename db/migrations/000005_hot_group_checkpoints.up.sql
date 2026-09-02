CREATE TABLE group_sync_states (
    group_uuid VARCHAR(24) NOT NULL,
    latest_message_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    latest_message_uuid VARCHAR(24) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (group_uuid),
    KEY idx_group_sync_states_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE device_group_sync_checkpoints (
    user_uuid VARCHAR(24) NOT NULL,
    device_id VARCHAR(128) NOT NULL,
    group_uuid VARCHAR(24) NOT NULL,
    pulled_message_seq BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_uuid, device_id, group_uuid),
    KEY idx_device_group_sync_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO group_sync_states (group_uuid, latest_message_seq, latest_message_uuid, created_at, updated_at)
SELECT message.target_uuid, message.seq, message.uuid, NOW(3), NOW(3)
FROM messages AS message
JOIN (
    SELECT target_uuid, MAX(seq) AS latest_message_seq
    FROM messages
    WHERE target_type = 1
    GROUP BY target_uuid
) AS latest
  ON latest.target_uuid = message.target_uuid
 AND latest.latest_message_seq = message.seq
WHERE message.target_type = 1;
