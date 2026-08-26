CREATE TABLE sync_inbox_baseline_jobs (
    job_name VARCHAR(128) NOT NULL,
    source_high_watermark_sync_seq BIGINT UNSIGNED NOT NULL,
    first_created_outbox_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    last_created_outbox_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    entry_count BIGINT UNSIGNED NOT NULL,
    entries_sha256 CHAR(64) NOT NULL,
    captured_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (job_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sync_inbox_baseline_entries (
    job_name VARCHAR(128) NOT NULL,
    sync_seq BIGINT UNSIGNED NOT NULL,
    user_uuid VARCHAR(24) NOT NULL,
    message_uuid VARCHAR(24) NOT NULL,
    conversation_key VARCHAR(64) NOT NULL,
    message_seq BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (job_name, sync_seq),
    UNIQUE KEY idx_sync_baseline_job_user_message (job_name, user_uuid, message_uuid),
    CONSTRAINT fk_sync_baseline_job FOREIGN KEY (job_name)
        REFERENCES sync_inbox_baseline_jobs (job_name) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
