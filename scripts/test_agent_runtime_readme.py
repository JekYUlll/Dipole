#!/usr/bin/env python3
"""Keep Agent Runtime deployment defaults aligned with the canonical Compose file."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parent.parent
README = (ROOT / "services/agent-runtime/README.md").read_text(encoding="utf-8")
COMPOSE = (ROOT / "deploy/compose/docker-compose.microservices.yml").read_text(encoding="utf-8")


class AgentRuntimeReadmeTest(unittest.TestCase):
    def test_readme_describes_current_compose_default(self) -> None:
        self.assertIn('DIPOLE_AGENT_RUNTIME_MODE: remote', COMPOSE)
        self.assertIn('DIPOLE_AGENT_TEMPORAL_ENABLED: "true"', COMPOSE)
        self.assertIn('DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: read_active', COMPOSE)
        self.assertIn('`remote + read_active` Durable Runtime', README)
        self.assertIn('主 Compose 保持 Temporal enabled + `read_active`', README)

    def test_readme_marks_shadow_as_an_explicit_profile(self) -> None:
        self.assertIn('DIPOLE_AGENT_RUNTIME_MODE: shadow', (ROOT / 'deploy/microservices/agent-temporal-read-shadow.yml').read_text(encoding='utf-8'))
        self.assertIn('`read_shadow` 是显式回退与测试 profile', README)
        self.assertNotIn('微服务 Compose 默认使用 `shadow`', README)


if __name__ == '__main__':
    unittest.main()
