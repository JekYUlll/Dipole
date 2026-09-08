-- Operator-only account for the offline context-ablation binding CLI.
-- Replace the password and restrict the host before applying after migration 000056.
CREATE USER IF NOT EXISTS 'dipole_agent_eval_bind'@'%' IDENTIFIED BY 'change-me';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'dipole_agent_eval_bind'@'%';

GRANT SELECT ON dipole.agent_tasks TO 'dipole_agent_eval_bind'@'%';
GRANT SELECT ON dipole.agent_runs TO 'dipole_agent_eval_bind'@'%';
GRANT SELECT, INSERT ON dipole.agent_context_ablation_bindings TO 'dipole_agent_eval_bind'@'%';

FLUSH PRIVILEGES;
