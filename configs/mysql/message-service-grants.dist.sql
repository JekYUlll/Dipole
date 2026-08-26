-- Replace the example password before applying this file.
CREATE USER IF NOT EXISTS 'dipole_message'@'%' IDENTIFIED BY 'change-me';

GRANT SELECT, INSERT, UPDATE, DELETE ON dipole.messages TO 'dipole_message'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON dipole.user_sync_inbox TO 'dipole_message'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON dipole.user_sync_states TO 'dipole_message'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON dipole.outbox_events TO 'dipole_message'@'%';
GRANT SELECT ON dipole.schema_migrations TO 'dipole_message'@'%';

FLUSH PRIVILEGES;
