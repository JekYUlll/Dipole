#!/usr/bin/env python3
"""Static contract for the opt-in parallel subscription experience worker."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentSubscriptionExperienceComposeTest(unittest.TestCase):
    def test_parallel_worker_is_explicit_and_read_only(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-subscription-experience.yml").read_text(encoding="utf-8")

        self.assertIn("  agent-subscription:\n", overlay)
        self.assertIn("      file: ../compose/docker-compose.microservices.yml", overlay)
        self.assertIn("      service: agent", overlay)
        self.assertIn("DIPOLE_AGENT_RELEASE_MANIFEST_FILE:?DIPOLE_AGENT_RELEASE_MANIFEST_FILE is required", overlay)
        self.assertIn("DIPOLE_AGENT_KAFKA_CLIENT_ID: dipole-agent-subscription", overlay)
        self.assertIn("DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_KAFKA_GROUP_ID:?", overlay)
        self.assertIn("DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_TASK_QUEUE:?", overlay)
        self.assertIn("DIPOLE_AGENT_MODEL_MAX_CALLS: ${DIPOLE_AGENT_MODEL_MAX_CALLS:-3}", overlay)
        self.assertIn("DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS: ${DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS:-180000}", overlay)
        self.assertIn("DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS: ${DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS:-16384}", overlay)
        self.assertIn("DIPOLE_AGENT_TRIGGER_MODE: subscription", overlay)
        self.assertIn("DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_ENABLED: \"true\"", overlay)
        self.assertIn("DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: subscription_active", overlay)

        for disabled in (
            "DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED: \"false\"",
            "DIPOLE_AGENT_INBOUND_INTERACTIVE_ENABLED: \"false\"",
            "DIPOLE_AGENT_INBOUND_GROUP_INTERACTIVE_ENABLED: \"false\"",
            "DIPOLE_AGENT_CONTROL_ENABLED: \"false\"",
            "DIPOLE_AGENT_INTERACTIVE_MESSAGE_WRITE_ENABLED: \"false\"",
            "DIPOLE_AGENT_SUBSCRIPTION_MESSAGE_WRITE_ENABLED: \"false\"",
            "DIPOLE_AGENT_MCP_SERVER_ENABLED: \"false\"",
            "DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: \"false\"",
        ):
            self.assertIn(disabled, overlay)


if __name__ == "__main__":
    unittest.main()
