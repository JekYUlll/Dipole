#!/usr/bin/env python3
"""Static guardrails for the authenticated first-party MCP smoke."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentMCPClientSmokeTest(unittest.TestCase):
    def test_smoke_uses_owner_bound_task_and_consent_grant(self) -> None:
        source = (ROOT / "scripts/smoke-agent-mcp-client.mjs").read_text(encoding="utf-8")
        self.assertIn('required("DIPOLE_MCP_BASE_URL")', source)
        self.assertIn('"/api/v1/agent/tasks"', source)
        self.assertIn("waitForMCPRun", source)
        self.assertIn('"/api/v1/auth/agent-mcp/token"', source)
        self.assertIn('"initialize"', source)
        self.assertIn('"tools/list"', source)
        self.assertIn('"dipole_conversation_list"', source)
        self.assertIn('headers.get("mcp-session-id")', source)
        self.assertNotIn("console.log(mcpToken", source)


if __name__ == "__main__":
    unittest.main()
