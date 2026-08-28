CREATE TABLE message_search_documents (
    message_uuid VARCHAR(24) NOT NULL,
    conversation_key VARCHAR(64) NOT NULL,
    message_seq BIGINT UNSIGNED NOT NULL,
    sender_uuid VARCHAR(24) NOT NULL,
    message_type TINYINT NOT NULL DEFAULT 0,
    content TEXT NOT NULL,
    sent_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (message_uuid),
    UNIQUE KEY idx_message_search_conversation_seq (conversation_key, message_seq),
    KEY idx_message_search_conversation_sent (conversation_key, sent_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
