#!/usr/bin/env python3
"""Static contract for the default-off interactive Agent shadow overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class InteractiveAgentShadowComposeTest(unittest.TestCase):
    def test_overlay_only_opens_control_over_read_shadow(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-interactive-shadow.yml").read_text(encoding="utf-8")
        self.assertIn("DIPOLE_AGENT_RUNTIME_MODE: shadow", overlay)
        self.assertIn("DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: read_shadow", overlay)
        self.assertIn('DIPOLE_AGENT_CONTROL_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_AGENT_MCP_SERVER_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_AGENT_MEMORY_ENABLED: "false"', overlay)

    def test_deepseek_overlay_keeps_the_interactive_profile_read_only(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-deepseek-v4-flash-shadow.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_MODEL_STRUCTURED_OUTPUTS: "false"', overlay)
        self.assertIn("DIPOLE_AGENT_MODEL_OUTPUT_MODE: json_text", overlay)
        self.assertIn("DIPOLE_AGENT_MODEL_THINKING_MODE: disabled", overlay)
        self.assertNotIn("gateway:", overlay)

    def test_compose_gate_checks_the_effective_profile(self) -> None:
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn("agent-interactive-shadow.yml", checker)
        self.assertIn("interactive_shadow_config", checker)
        self.assertIn('DIPOLE_AGENT_RUNTIME_MODE == "shadow"', checker)
        self.assertIn('DIPOLE_AGENT_CONTROL_ENABLED == "true"', checker)
        self.assertIn('DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED == "true"', checker)
        interactive_shadow = checker.split("interactive_shadow_config", 1)[1].split("interactive_active_config", 1)[0]
        self.assertIn("DIPOLE_GATEWAY_AGENT_CONTROL_SECRET=compose-check-control-secret", interactive_shadow)


if __name__ == "__main__":
    unittest.main()
