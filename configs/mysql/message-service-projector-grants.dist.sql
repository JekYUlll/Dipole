-- Replace the example password before applying this file after migrations.
-- This account is valid only with message.inbox_write_mode=projector.
CREATE USER IF NOT EXISTS 'dipole_message_projector'@'%' IDENTIFIED BY 'change-me';

GRANT SELECT, INSERT, UPDATE, DELETE ON dipole.messages TO 'dipole_message_projector'@'%';
GRANT SELECT, INSERT ON dipole.message_metadata TO 'dipole_message_projector'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.conversation_sequences TO 'dipole_message_projector'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.group_sync_states TO 'dipole_message_projector'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON dipole.outbox_events TO 'dipole_message_projector'@'%';
GRANT SELECT ON dipole.schema_migrations TO 'dipole_message_projector'@'%';
-- Temporary read compatibility for /messages/offline until clients migrate to /sync.
GRANT SELECT ON dipole.groups TO 'dipole_message_projector'@'%';
GRANT SELECT ON dipole.group_members TO 'dipole_message_projector'@'%';

FLUSH PRIVILEGES;
