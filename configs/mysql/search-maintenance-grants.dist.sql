-- Replace the example password before applying this file.
CREATE USER IF NOT EXISTS 'dipole_search_maintenance'@'%' IDENTIFIED BY 'change-me';

GRANT SELECT ON dipole.schema_migrations TO 'dipole_search_maintenance'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.search_backfill_jobs TO 'dipole_search_maintenance'@'%';
GRANT SELECT, DELETE ON dipole.outbox_events TO 'dipole_search_maintenance'@'%';

FLUSH PRIVILEGES;
