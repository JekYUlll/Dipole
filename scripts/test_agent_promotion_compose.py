#!/usr/bin/env python3
"""Keep the operator-only promotion control disabled in the default Compose path."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parent.parent
COMPOSE = (ROOT / "deploy/compose/docker-compose.microservices.yml").read_text(encoding="utf-8")
CONFIG = (ROOT / "configs/config.dist.yaml").read_text(encoding="utf-8")


class AgentPromotionComposeTest(unittest.TestCase):
    def test_default_compose_keeps_operator_route_disabled(self) -> None:
        self.assertIn('DIPOLE_GATEWAY_AGENT_PROMOTION_ENABLED: ${DIPOLE_GATEWAY_AGENT_PROMOTION_ENABLED:-false}', COMPOSE)
        self.assertIn('DIPOLE_GATEWAY_AGENT_PROMOTION_TENANT_ID: ${DIPOLE_GATEWAY_AGENT_PROMOTION_TENANT_ID:-dipole}', COMPOSE)

    def test_sample_config_explains_core_operator_guards(self) -> None:
        self.assertIn('agent_promotion_enabled: false', CONFIG)
        self.assertIn('agent_promotion_tenant_id: dipole', CONFIG)
        self.assertIn('durable operator grant and two distinct principals', CONFIG)


if __name__ == '__main__':
    unittest.main()
