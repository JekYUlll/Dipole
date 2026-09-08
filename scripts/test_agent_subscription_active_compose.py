#!/usr/bin/env python3
"""Static safety contract for the isolated subscription-active Compose smoke."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentSubscriptionActiveComposeSmokeTest(unittest.TestCase):
    def test_smoke_isolated_and_deterministic(self) -> None:
        smoke_path = ROOT / "scripts/smoke-agent-subscription-active-compose.sh"
        smoke = smoke_path.read_text(encoding="utf-8")
        self.assertNotEqual(smoke_path.stat().st_mode & 0o111, 0)
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
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_PROMOTION_MODE:-fixture', smoke)
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_PROMOTION_MODE must be fixture, control, or publication', smoke)
        self.assertIn('DIPOLE_GATEWAY_AGENT_PROMOTION_ENABLED=true', smoke)

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
        self.assertIn('subscription task completed without a model call', smoke)
        self.assertIn('"${model_calls}" -ge 1', smoke)
        self.assertIn('subscription read task wrote', smoke)

    def test_control_mode_uses_gateway_proposal_and_second_review(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn('/api/v1/agent/runtime-promotions', smoke)
        self.assertIn('/review', smoke)
        self.assertIn('promotion propose failed', smoke)
        self.assertIn('promotion review failed', smoke)
        self.assertIn('const grantValidFromUnixMs = now + 2000;', smoke)
        self.assertIn('grantValidFromUnixMs - Date.now() + 100', smoke)
        self.assertNotIn('grantValidFromUnixMs: now - 1000', smoke)
        self.assertIn('agent_runtime_promotion_operator_grants', smoke)
        self.assertIn("'shadow', 'completed'", smoke)
        self.assertIn('evidence-task', smoke)
        self.assertIn('evidence-run', smoke)
        self.assertNotIn('evidence_task="task:', smoke)
        self.assertNotIn('evidence_run="run:', smoke)
        self.assertIn('agent-runtime.subscription-active-compose-smoke', smoke)
        self.assertNotIn('agent-runtime@subscription-active-compose-smoke', smoke)

    def test_publication_mode_uses_runtime_artifact_rpc_and_receipt(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn('"${promotion_mode}" == "publication"', smoke)
        self.assertIn('PromotionEvidencePublisher', smoke)
        self.assertIn('createAgentCapabilityRPC', smoke)
        self.assertIn('DIPOLE_AGENT_CAPABILITY_RPC_ENABLED=true', smoke)
        self.assertIn('promotion_evaluation', smoke)
        self.assertIn('publication_receipt', smoke)
        self.assertIn('receipt.artifactId', smoke)
        self.assertIn('receipt.evidenceSHA256', smoke)

    def test_model_stub_stays_inside_the_compose_project(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-subscription-active-smoke.yml").read_text(encoding="utf-8")
        smoke = (ROOT / "scripts/smoke-agent-subscription-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_MODEL_STUB_FILE', overlay)
        self.assertIn('entrypoint: ["/bin/sh", "-ec"]', overlay)
        self.assertIn('node /app/model-stub.mjs & exec node dist/index.js', overlay)
        self.assertIn('requestBody?.response_format?.json_schema?.schema', smoke)
        self.assertIn('schema?.properties?.steps !== undefined', smoke)
        self.assertIn('raw.includes("steps")', smoke)
        self.assertIn('expectsPlan ? { summary, steps: [] } : { summary }', smoke)

    def test_provider_mode_uses_a_protected_env_file_and_read_only_overlays(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-subscription-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn('model_source="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_MODEL_SOURCE:-stub}"', smoke)
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_MODEL_ENV_FILE is required for provider mode', smoke)
        self.assertIn('agent-ai-sdk-shadow.yml', smoke)
        self.assertIn('agent-deepseek-v4-flash-shadow.yml', smoke)
        self.assertIn('docker compose "${env_args[@]}" -p "${project_name}"', smoke)


if __name__ == "__main__":
    unittest.main()
