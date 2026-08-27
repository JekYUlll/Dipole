CREATE TABLE cassandra_backfill_jobs (
    job_name VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    source_high_watermark_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    last_processed_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    owner_id VARCHAR(128) NOT NULL DEFAULT '',
    lease_expires_at DATETIME(3) NULL,
    attempt_count BIGINT UNSIGNED NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL,
    completed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (job_name),
    KEY idx_cassandra_backfill_status_lease (status, lease_expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
