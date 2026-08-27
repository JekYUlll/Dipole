ALTER TABLE cassandra_backfill_jobs
    ADD COLUMN source_kind VARCHAR(64) NOT NULL DEFAULT 'mysql_messages' AFTER job_name,
    ADD COLUMN source_snapshot_id VARCHAR(255) NOT NULL DEFAULT '' AFTER source_kind,
    ADD COLUMN source_sha256 CHAR(64) NOT NULL DEFAULT '' AFTER source_snapshot_id;

UPDATE cassandra_backfill_jobs
SET source_snapshot_id = CONCAT('mysql-messages:', source_high_watermark_id)
WHERE source_snapshot_id = '';
