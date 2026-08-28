-- Replace the example password and restrict the host before applying this file after migrations.
CREATE USER IF NOT EXISTS 'dipole_agent_eval'@'%' IDENTIFIED BY 'change-me';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'dipole_agent_eval'@'%';

GRANT SELECT ON dipole.agent_tasks TO 'dipole_agent_eval'@'%';
GRANT SELECT ON dipole.agent_runs TO 'dipole_agent_eval'@'%';
GRANT SELECT ON dipole.agent_shadow_plans TO 'dipole_agent_eval'@'%';
GRANT SELECT ON dipole.agent_shadow_steps TO 'dipole_agent_eval'@'%';
GRANT SELECT ON dipole.agent_artifacts TO 'dipole_agent_eval'@'%';
GRANT SELECT ON dipole.agent_model_runs TO 'dipole_agent_eval'@'%';
GRANT SELECT ON dipole.agent_model_calls TO 'dipole_agent_eval'@'%';
GRANT SELECT ON dipole.agent_tool_invocations TO 'dipole_agent_eval'@'%';
GRANT SELECT ON dipole.agent_memories TO 'dipole_agent_eval'@'%';
GRANT SELECT ON dipole.agent_memory_task_lineage TO 'dipole_agent_eval'@'%';

FLUSH PRIVILEGES;
