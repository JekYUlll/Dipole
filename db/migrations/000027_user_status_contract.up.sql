UPDATE users
SET status = 1
WHERE status = 0;

ALTER TABLE users
    ALTER COLUMN status SET DEFAULT 1,
    ADD CONSTRAINT chk_users_status_v1 CHECK (status IN (1, 2));
