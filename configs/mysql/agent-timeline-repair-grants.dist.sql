-- Replace the example password before applying this file after migrations.
CREATE USER IF NOT EXISTS 'dipole_agent_timeline_repair'@'%' IDENTIFIED BY 'change-me';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'dipole_agent_timeline_repair'@'%';

GRANT SELECT, INSERT ON dipole.agent_task_timeline_events TO 'dipole_agent_timeline_repair'@'%';
GRANT SELECT, INSERT, UPDATE ON dipole.agent_task_timeline_repairs TO 'dipole_agent_timeline_repair'@'%';
GRANT SELECT ON dipole.schema_migrations TO 'dipole_agent_timeline_repair'@'%';

FLUSH PRIVILEGES;
