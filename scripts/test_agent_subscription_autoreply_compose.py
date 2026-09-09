#!/usr/bin/env python3
"""Static contract for the explicit subscription auto-reply Agent overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class SubscriptionAutoReplyComposeTest(unittest.TestCase):
    def test_overlay_only_flips_the_write_flag_default_off(self) -> None:
        overlay = (
            ROOT / "deploy/microservices/agent-subscription-autoreply.yml"
        ).read_text(encoding="utf-8")
        self.assertIn(
            'DIPOLE_AGENT_SUBSCRIPTION_MESSAGE_WRITE_ENABLED: "true"', overlay
        )
        # The opt-in must not smuggle in any other capability toggle; every other
        # invariant is inherited from agent-active + agent-subscription-active.
        self.assertNotIn("DIPOLE_AGENT_CONTROL_ENABLED", overlay)
        self.assertNotIn("DIPOLE_AGENT_MCP_SERVER_ENABLED", overlay)
        self.assertNotIn("DIPOLE_GATEWAY_AGENT", overlay)

    def test_subscription_active_keeps_auto_reply_off_by_default(self) -> None:
        base = (
            ROOT / "deploy/microservices/agent-subscription-active.yml"
        ).read_text(encoding="utf-8")
        self.assertIn(
            'DIPOLE_AGENT_SUBSCRIPTION_MESSAGE_WRITE_ENABLED: "false"', base
        )

    def test_compose_gate_checks_both_profiles(self) -> None:
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn("agent-subscription-autoreply.yml", checker)
        self.assertIn("subscription_autoreply_config", checker)
        self.assertIn(
            'DIPOLE_AGENT_SUBSCRIPTION_MESSAGE_WRITE_ENABLED == "true"', checker
        )
        self.assertIn(
            'DIPOLE_AGENT_SUBSCRIPTION_MESSAGE_WRITE_ENABLED == "false"', checker
        )
        self.assertIn('DIPOLE_AGENT_CAPABILITY_RPC_TARGET == "core:9091"', checker)

    def test_smoke_uses_the_public_autoreply_definition_profile(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-autoreply-compose.sh").read_text(encoding="utf-8")
        self.assertIn('profile: "subscription_autoreply"', smoke)
        self.assertIn('JSON.stringify({ profile: "subscription_autoreply" })', smoke)
        self.assertNotIn('UPDATE agent_definition_versions', smoke)

    def test_smoke_stub_matches_plan_and_summary_schemas(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-autoreply-compose.sh").read_text(encoding="utf-8")
        self.assertIn("requestBody?.response_format?.json_schema?.schema", smoke)
        self.assertIn("expectsPlan", smoke)
        self.assertIn('? { summary: "subscription autoreply smoke reply", steps: [] }', smoke)
        self.assertIn(': { summary: "subscription autoreply smoke reply" }', smoke)

    def test_smoke_locks_the_bounded_two_call_runtime_path(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-autoreply-compose.sh").read_text(encoding="utf-8")
        self.assertIn("expected_model_calls=2", smoke)
        self.assertIn('[[ "${model_calls}" == "${expected_model_calls}" ]]', smoke)

    def test_smoke_replays_the_published_outbox_envelope_without_extra_effects(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-autoreply-compose.sh").read_text(encoding="utf-8")
        self.assertIn("mysql -N -B -r", smoke)
        self.assertIn("REPLACE(TO_BASE64(value), CHAR(10), '')", smoke)
        self.assertIn("kafka-console-producer.sh", smoke)
        self.assertIn('"dipole.${replay_topic}"', smoke)
        self.assertIn("subscription Kafka replay diverged", smoke)
        self.assertIn("agent_event_ledger", smoke)


if __name__ == "__main__":
    unittest.main()
