ALTER TABLE message_search_documents
    MODIFY conversation_key VARCHAR(64) NULL,
    MODIFY message_seq BIGINT UNSIGNED NULL,
    MODIFY sender_uuid VARCHAR(24) NULL,
    MODIFY message_type TINYINT NULL,
    MODIFY content TEXT NULL,
    MODIFY sent_at DATETIME(3) NULL,
    ADD COLUMN revision BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER message_seq,
    ADD COLUMN searchable BOOLEAN NOT NULL DEFAULT TRUE AFTER sent_at,
    ADD COLUMN payload_hash CHAR(64) NOT NULL DEFAULT '' AFTER searchable;
