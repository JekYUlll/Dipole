import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class AgentTimelineRepairComposeContractTest(unittest.TestCase):
    def test_repair_worker_is_opt_in_and_has_persisted_dependencies(self):
        compose = (ROOT / "docker-compose.microservices.yml").read_text(encoding="utf-8")
        dockerfile = (ROOT / "Dockerfile").read_text(encoding="utf-8")
        build = (ROOT / "scripts/docker-build.sh").read_text(encoding="utf-8")
        smoke = (ROOT / "scripts/smoke-agent-timeline-repair.sh").read_text(encoding="utf-8")
        compose_smoke = (ROOT / "scripts/smoke-agent-timeline-repair-compose.sh").read_text(encoding="utf-8")

        self.assertIn('agent-timeline-repair:', compose)
        self.assertIn('profiles: ["agent-timeline-repair"]', compose)
        self.assertIn('/app/dipole-agent-task-timeline-repair', compose)
        self.assertIn('mysql-permissions:', compose)
        self.assertIn('condition: service_completed_successfully', compose)
        self.assertIn('COPY dist/dipole-agent-task-timeline-repair', dockerfile)
        self.assertIn('./cmd/agent-task-timeline-repair', build)
        self.assertIn('-once', smoke)
        self.assertIn('agent_task_timeline_repairs', smoke)
        self.assertIn('agent_task_timeline_events', smoke)
        self.assertIn('ALTER USER', compose)
        self.assertIn('DIPOLE_AGENT_TIMELINE_REPAIR_MYSQL_PASSWORD', compose)
        self.assertIn('unsupported SQL quoting characters', compose)
        self.assertIn('agent-timeline-repair', compose_smoke)
        self.assertIn('compose up -d --wait mysql', compose_smoke)
        self.assertIn('compose run --rm --no-deps migrate', compose_smoke)
        self.assertIn('migration preflight failed', compose_smoke)
        self.assertIn('timeline_table', compose_smoke)
        self.assertIn('@@global.time_zone, @@session.time_zone', compose_smoke)
        self.assertIn('timezone_state', compose_smoke)
        self.assertIn('compose up -d --wait mysql-permissions', compose_smoke)
        self.assertIn('pending_state', compose_smoke)
        self.assertIn('/readyz', compose_smoke)
        self.assertIn('EVENT-SMOKE-COMPOSE-REPAIR', compose_smoke)


if __name__ == "__main__":
    unittest.main()
