-- Expand-only compatibility migration. Keeping VARCHAR(24) is safe for the
-- previous application and avoids truncating valid 21-24 character identities.
SELECT 1;
