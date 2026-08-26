-- Replace the example password before applying this file after migrations.
CREATE USER IF NOT EXISTS 'dipole_sync'@'%' IDENTIFIED BY 'change-me';

GRANT SELECT ON dipole.messages TO 'dipole_sync'@'%';
GRANT SELECT ON dipole.outbox_events TO 'dipole_sync'@'%';
GRANT SELECT ON dipole.group_sync_states TO 'dipole_sync'@'%';
GRANT SELECT ON dipole.schema_migrations TO 'dipole_sync'@'%';

GRANT SELECT, INSERT, UPDATE ON dipole.user_sync_inbox TO 'dipole_sync'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.user_sync_states TO 'dipole_sync'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.device_sync_checkpoints TO 'dipole_sync'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.device_group_sync_checkpoints TO 'dipole_sync'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.sync_replay_jobs TO 'dipole_sync'@'%';
GRANT SELECT, INSERT ON dipole.sync_inbox_baseline_jobs TO 'dipole_sync'@'%';
GRANT SELECT, INSERT ON dipole.sync_inbox_baseline_entries TO 'dipole_sync'@'%';

FLUSH PRIVILEGES;
