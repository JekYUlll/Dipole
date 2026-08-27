ALTER TABLE cassandra_backfill_jobs
    DROP COLUMN source_sha256,
    DROP COLUMN source_snapshot_id,
    DROP COLUMN source_kind;
