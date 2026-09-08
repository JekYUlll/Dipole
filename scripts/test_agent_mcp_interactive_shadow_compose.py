#!/usr/bin/env python3
"""Static contract for the combined interactive MCP shadow overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentMcpInteractiveShadowComposeTest(unittest.TestCase):
    def test_overlay_restores_only_the_explicit_combined_surface(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-mcp-interactive-shadow.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_CONTROL_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_AGENT_MCP_SERVER_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_ARTIFACT_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_MCP_ENABLED: "true"', overlay)

    def test_compose_gate_includes_the_final_overlay(self) -> None:
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn("agent-mcp-interactive-shadow.yml", checker)


if __name__ == "__main__":
    unittest.main()
