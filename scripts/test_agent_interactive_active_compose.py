#!/usr/bin/env python3
"""Static contract for the explicit interactive active Agent overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class InteractiveAgentActiveComposeTest(unittest.TestCase):
    def test_overlay_exposes_task_control_and_read_only_definition_routes(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-interactive-active.yml").read_text(encoding="utf-8")
        self.assertIn("DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: interactive_active", overlay)
        self.assertIn('DIPOLE_AGENT_CONTROL_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_MESSAGE_WRITE_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_CONTROL_SECRET: ${DIPOLE_AGENT_CONTROL_SECRET:', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_ARTIFACT_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_MCP_ENABLED: "false"', overlay)

    def test_compose_gate_checks_the_effective_profile(self) -> None:
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn("agent-interactive-active.yml", checker)
        self.assertIn("interactive_active_config", checker)
        self.assertIn('DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED == "true"', checker)
        self.assertIn('DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED == "true"', checker)
        self.assertIn('DIPOLE_GATEWAY_AGENT_ARTIFACT_ENABLED == "false"', checker)


if __name__ == "__main__":
    unittest.main()
