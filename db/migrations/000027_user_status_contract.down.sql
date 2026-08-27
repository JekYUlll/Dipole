ALTER TABLE users
    DROP CHECK chk_users_status_v1,
    ALTER COLUMN status SET DEFAULT 0;
