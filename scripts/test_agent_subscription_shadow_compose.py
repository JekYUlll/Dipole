#!/usr/bin/env python3
"""Static contract for the opt-in Agent Subscription Shadow Compose overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentSubscriptionShadowComposeTest(unittest.TestCase):
    def test_overlay_keeps_direct_target_and_only_enables_observation(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-subscription-shadow.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_RUNTIME_MODE: shadow', overlay)
        self.assertIn('DIPOLE_AGENT_TRIGGER_MODE: direct_target', overlay)
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_SUBSCRIPTION_ENABLED: "true"', overlay)
        for key in (
            'DIPOLE_AGENT_MEMORY_ENABLED: "false"',
            'DIPOLE_AGENT_RETRIEVAL_ENABLED: "false"',
            'DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED: "false"',
            'DIPOLE_AGENT_CONTROL_ENABLED: "false"',
            'DIPOLE_AGENT_MCP_SERVER_ENABLED: "false"',
            'DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "false"',
            'DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED: "false"',
            'DIPOLE_GATEWAY_AGENT_MCP_ENABLED: "false"',
        ):
            self.assertIn(key, overlay)

    def test_compose_gate_checks_the_effective_profile(self) -> None:
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn('subscription_shadow_config="$(', checker)
        self.assertIn('agent-subscription-shadow.yml', checker)
        profile = checker.split('subscription_shadow_config="$(', 1)[1].split('interactive_active_config', 1)[0]
        self.assertIn('DIPOLE_AGENT_TRIGGER_MODE == "direct_target"', profile)
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED == "true"', profile)
        self.assertIn('DIPOLE_GATEWAY_AGENT_SUBSCRIPTION_ENABLED == "true"', profile)
        self.assertIn('DIPOLE_AGENT_CONTROL_ENABLED == "false"', profile)
        self.assertIn('DIPOLE_AGENT_MCP_SERVER_ENABLED == "false"', profile)

    def test_smoke_establishes_an_authorized_owner_conversation(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-shadow-compose.sh").read_text(encoding="utf-8")
        self.assertIn('type: "chat.send"', smoke)
        self.assertIn('target_uuid: agentUuid', smoke)
        self.assertIn('/api/v1/agent/subscriptions/options?', smoke)
        self.assertIn('bootstrap direct conversation did not become eligible for subscription', smoke)
        self.assertLess(smoke.index('await sendBootstrapMessage();'), smoke.index('const subscriptionResponse ='))


if __name__ == "__main__":
    unittest.main()
