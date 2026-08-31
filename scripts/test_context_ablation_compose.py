import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OVERLAYS = {
    "baseline": {"DIPOLE_AGENT_MEMORY_ENABLED: \"false\"", "DIPOLE_AGENT_RETRIEVAL_ENABLED: \"false\"", "DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED: \"false\""},
    "retrieval": {"DIPOLE_AGENT_MEMORY_ENABLED: \"false\"", "DIPOLE_AGENT_RETRIEVAL_ENABLED: \"true\"", "DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED: \"true\""},
    "memory": {"DIPOLE_AGENT_MEMORY_ENABLED: \"true\"", "DIPOLE_AGENT_RETRIEVAL_ENABLED: \"false\"", "DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED: \"false\""},
}


class ContextAblationComposeContractTest(unittest.TestCase):
    def test_conditions_are_opt_in_and_mutually_distinct(self):
        queues = set()
        for condition, expected in OVERLAYS.items():
            source = (ROOT / "deploy/microservices" / f"agent-context-ablation-{condition}.yml").read_text(encoding="utf-8")
            for setting in expected:
                self.assertIn(setting, source)
            queue = f"dipole-agent-context-ablation-{condition}-v1"
            self.assertIn(f"DIPOLE_AGENT_TEMPORAL_TASK_QUEUE: {queue}", source)
            queues.add(queue)
        self.assertEqual(len(queues), 3)

    def test_base_read_shadow_remains_disabled_for_optional_context_sources(self):
        source = (ROOT / "deploy/microservices/agent-temporal-read-shadow.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_MEMORY_ENABLED: "false"', source)
        self.assertIn('DIPOLE_AGENT_RETRIEVAL_ENABLED: "false"', source)
        self.assertIn('DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED: "false"', source)


if __name__ == "__main__":
    unittest.main()
