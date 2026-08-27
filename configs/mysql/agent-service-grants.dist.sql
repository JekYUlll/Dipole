-- Replace the example password before applying this file after migrations.
CREATE USER IF NOT EXISTS 'dipole_agent'@'%' IDENTIFIED BY 'change-me';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'dipole_agent'@'%';

GRANT SELECT ON dipole.schema_migrations TO 'dipole_agent'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.agent_event_ledger TO 'dipole_agent'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.agent_model_runs TO 'dipole_agent'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.agent_model_calls TO 'dipole_agent'@'%';

FLUSH PRIVILEGES;
