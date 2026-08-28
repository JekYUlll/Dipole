import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class AgentTimelineRepairComposeContractTest(unittest.TestCase):
    def test_repair_worker_is_opt_in_and_has_persisted_dependencies(self):
        compose = (ROOT / "docker-compose.microservices.yml").read_text(encoding="utf-8")
        dockerfile = (ROOT / "Dockerfile").read_text(encoding="utf-8")
        build = (ROOT / "scripts/docker-build.sh").read_text(encoding="utf-8")

        self.assertIn('agent-timeline-repair:', compose)
        self.assertIn('profiles: ["agent-timeline-repair"]', compose)
        self.assertIn('/app/dipole-agent-task-timeline-repair', compose)
        self.assertIn('mysql-permissions:', compose)
        self.assertIn('condition: service_completed_successfully', compose)
        self.assertIn('COPY dist/dipole-agent-task-timeline-repair', dockerfile)
        self.assertIn('./cmd/agent-task-timeline-repair', build)


if __name__ == "__main__":
    unittest.main()
