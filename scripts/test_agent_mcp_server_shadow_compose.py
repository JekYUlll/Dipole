#!/usr/bin/env python3
"""Static safety contract for the explicit first-party MCP Server overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentMcpServerShadowComposeTest(unittest.TestCase):
    def test_overlay_adds_only_the_read_only_mcp_surface(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-mcp-server-shadow.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_MCP_SERVER_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_MCP_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_AGENT_MEMORY_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_AGENT_RETRIEVAL_ENABLED: "false"', overlay)
        self.assertNotIn("DIPOLE_AGENT_CONTROL_ENABLED", overlay)
        self.assertNotIn("DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED", overlay)
        self.assertNotIn("DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED", overlay)
        self.assertNotIn("DIPOLE_GATEWAY_AGENT_ARTIFACT_ENABLED", overlay)

    def test_compose_gate_checks_effective_mcp_profile(self) -> None:
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn("agent-mcp-server-shadow.yml", checker)
        self.assertIn("mcp_server_shadow_config", checker)
        self.assertIn('DIPOLE_AGENT_MCP_SERVER_ENABLED == "true"', checker)
        self.assertIn('DIPOLE_GATEWAY_AGENT_MCP_ENABLED == "true"', checker)


if __name__ == "__main__":
    unittest.main()
