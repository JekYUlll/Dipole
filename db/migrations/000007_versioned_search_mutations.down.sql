DELETE FROM message_search_documents WHERE searchable = FALSE;

ALTER TABLE message_search_documents
    MODIFY conversation_key VARCHAR(64) NOT NULL,
    MODIFY message_seq BIGINT UNSIGNED NOT NULL,
    MODIFY sender_uuid VARCHAR(24) NOT NULL,
    MODIFY message_type TINYINT NOT NULL DEFAULT 0,
    MODIFY content TEXT NOT NULL,
    MODIFY sent_at DATETIME(3) NOT NULL,
    DROP COLUMN payload_hash,
    DROP COLUMN searchable,
    DROP COLUMN revision;
