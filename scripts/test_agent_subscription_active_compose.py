#!/usr/bin/env python3
"""Static safety contract for the isolated subscription-active Compose smoke."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentSubscriptionActiveComposeSmokeTest(unittest.TestCase):
    def test_smoke_isolated_and_deterministic(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn('project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-subscription-active-', smoke)
        self.assertIn('DIPOLE_GATEWAY_BIND_ADDRESS:=127.0.0.1', smoke)
        self.assertIn('DIPOLE_AGENT_TEMPORAL_ADDRESS:=temporal:7233', smoke)
        self.assertIn('DIPOLE_AGENT_TEMPORAL_TASK_QUEUE:=${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_TASK_QUEUE}', smoke)
        self.assertIn('DIPOLE_MYSQL_AIO_COMPAT:=0', smoke)
        self.assertIn('remote-gpu-mysql-aio-compat.yml', smoke)
        self.assertIn('agent-subscription-active-smoke.yml', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_BASE_URL="http://127.0.0.1:8089/v1"', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_API_KEY="compose-smoke-no-network"', smoke)
        self.assertIn('DIPOLE_MICROSERVICE_IMAGE_SERVICES="migrate,core,gateway,message,sync"', smoke)
        self.assertIn('device=smoke-subscription', smoke)
        self.assertNotIn('from "kafkajs"', smoke)
        self.assertIn('compose down --volumes --remove-orphans', smoke)
        self.assertIn('Subscription active Compose stack retained: project=%s scratch=%s', smoke)
        self.assertIn('UPDATE agent_runtime_promotion_grants SET revoked_at', smoke)

    def test_autoreply_requires_explicit_opt_in_and_asserts_exact_side_effects(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn(': "${DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY:=0}"', smoke)
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_AUTOREPLY must be 0 or 1', smoke)
        self.assertIn('agent-subscription-autoreply.yml', smoke)
        self.assertIn('profile: "subscription_autoreply"', smoke)
        self.assertIn("subscription auto-reply side effects drifted", smoke)
        self.assertIn("capability_id = 'message.system.send'", smoke)
        self.assertIn("$'1\\t1\\t1\\t1\\t2'", smoke)

    def test_smoke_covers_owner_subscription_and_read_only_terminal_state(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn('/api/v1/agent/subscriptions/options?', smoke)
        self.assertIn('/api/v1/agent/subscriptions', smoke)
        self.assertIn('trigger_subscription_uuid', smoke)
        self.assertIn('"completed:completed"', smoke)
        self.assertIn('expected one subscription task', smoke)
        self.assertIn('expected one completed model call', smoke)
        self.assertIn('subscription read task wrote', smoke)

    def test_model_stub_stays_inside_the_compose_project(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-subscription-active-smoke.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_MODEL_STUB_FILE', overlay)
        self.assertIn('entrypoint: ["/bin/sh", "-ec"]', overlay)
        self.assertIn('node /app/model-stub.mjs & exec node dist/index.js', overlay)


if __name__ == "__main__":
    unittest.main()
