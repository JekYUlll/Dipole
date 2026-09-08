#!/usr/bin/env python3
"""Static contract for the opt-in public promotion-control maintenance overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentPromotionExperienceComposeTest(unittest.TestCase):
    def test_overlay_only_enables_gateway_operator_route(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-promotion-experience.yml").read_text(encoding="utf-8")

        self.assertIn("  gateway:\n", overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_PROMOTION_ENABLED: "true"', overlay)
        self.assertIn(
            "DIPOLE_GATEWAY_AGENT_PROMOTION_TENANT_ID: ${DIPOLE_GATEWAY_AGENT_PROMOTION_TENANT_ID:?DIPOLE_GATEWAY_AGENT_PROMOTION_TENANT_ID is required}",
            overlay,
        )
        self.assertNotIn("  agent:", overlay)
        self.assertNotIn("DIPOLE_AGENT_SUBSCRIPTION_MESSAGE_WRITE_ENABLED", overlay)


if __name__ == "__main__":
    unittest.main()
