#!/usr/bin/env python3
"""Keep the Retrieval experience overlay narrow and opt-in."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]
OVERLAY = ROOT / "deploy/microservices/agent-retrieval-experience.yml"


class AgentRetrievalExperienceComposeTest(unittest.TestCase):
    def test_only_core_and_agent_are_overridden(self) -> None:
        source = OVERLAY.read_text(encoding="utf-8")
        self.assertIn("  core:\n", source)
        self.assertIn("  agent:\n", source)
        self.assertNotIn("  gateway:\n", source)
        self.assertNotIn("  message:\n", source)
        self.assertNotIn("  sync:\n", source)

    def test_enables_governed_search_at_both_authority_points(self) -> None:
        source = OVERLAY.read_text(encoding="utf-8")
        self.assertIn('DIPOLE_INTERNAL_RPC_AGENT_CONVERSATION_SEARCH_ENABLED: "true"', source)
        self.assertIn('DIPOLE_AGENT_RETRIEVAL_ENABLED: "true"', source)
        self.assertIn('DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED: "true"', source)


if __name__ == "__main__":
    unittest.main()
