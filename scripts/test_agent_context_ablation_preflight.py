import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class ContextAblationPreflightContractTest(unittest.TestCase):
    def test_preflight_uses_disposable_mysql_and_checks_read_only_access(self):
        source = (ROOT / "scripts/smoke-agent-context-ablation-preflight.sh").read_text(encoding="utf-8")
        self.assertIn('project="dipole-agent-context-ablation-preflight-', source)
        self.assertIn('docker network create "$network"', source)
        self.assertIn('docker rm -f "$container"', source)
        self.assertIn('docker network rm "$network"', source)
        self.assertIn('version = 56', source)
        self.assertIn("agent_context_ablation_bindings", source)
        self.assertIn("dipole_agent_eval", source)
        self.assertIn("privilege_type IN ('INSERT', 'UPDATE', 'DELETE')", source)
        self.assertNotIn("docker compose", source)

    def test_binding_migration_matches_agent_identity_collation(self):
        source = (ROOT / "db/migrations/000056_agent_context_ablation_bindings.up.sql").read_text(encoding="utf-8")
        self.assertIn("ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci", source)
        self.assertIn("REFERENCES agent_tasks(task_uuid)", source)
        self.assertIn("REFERENCES agent_runs(run_uuid)", source)


if __name__ == "__main__":
    unittest.main()
