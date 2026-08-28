ALTER TABLE search_backfill_jobs
    ADD COLUMN source_kind VARCHAR(64) NOT NULL DEFAULT 'mysql_outbox' AFTER target_index,
    ADD COLUMN source_snapshot_id VARCHAR(255) NOT NULL DEFAULT '' AFTER source_kind,
    ADD COLUMN source_sha256 CHAR(64) NOT NULL DEFAULT '' AFTER source_snapshot_id;

UPDATE search_backfill_jobs
SET source_snapshot_id = CONCAT('mysql-outbox:', source_high_watermark_id)
WHERE source_snapshot_id = '';
