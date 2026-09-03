#!/usr/bin/env python3
"""Static safety contract for the interactive Agent Task Compose smoke."""

import os
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentInteractiveShadowComposeSmokeTest(unittest.TestCase):
    def test_smoke_is_executable_from_documented_command(self) -> None:
        self.assertTrue(os.access(ROOT / "scripts/smoke-agent-interactive-shadow-compose.sh", os.X_OK))

    def test_smoke_isolated_and_loopback_only(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-interactive-shadow-compose.sh").read_text(encoding="utf-8")
        self.assertIn('project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-interactive-shadow-', smoke)
        self.assertIn('DIPOLE_GATEWAY_BIND_ADDRESS:=127.0.0.1', smoke)
        self.assertIn('DIPOLE_AGENT_TEMPORAL_ADDRESS:=temporal:7233', smoke)
        self.assertIn('DIPOLE_AGENT_KAFKA_GROUP_ID:=dipole-agent-shadow-interactive-', smoke)
        self.assertIn('DIPOLE_MYSQL_AIO_COMPAT:=0', smoke)
        self.assertIn('remote-gpu-mysql-aio-compat.yml', smoke)
        self.assertIn('agent-interactive-shadow-smoke.yml', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_BASE_URL="http://127.0.0.1:8089/v1"', smoke)
        self.assertIn('compose down --volumes --remove-orphans', smoke)

    def test_provider_mode_requires_an_env_file_and_keeps_shadow_overlays(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-interactive-shadow-compose.sh").read_text(encoding="utf-8")
        self.assertIn('model_source="${DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_SOURCE:-stub}"', smoke)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_SOURCE must be stub or provider', smoke)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_ENV_FILE is required for provider mode', smoke)
        self.assertLess(
            smoke.index('DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_SOURCE must be stub or provider'),
            smoke.index('scratch_dir=$(mktemp -d')
        )
        self.assertIn('agent-ai-sdk-shadow.yml', smoke)
        self.assertIn('agent-deepseek-v4-flash-shadow.yml', smoke)
        self.assertIn('docker compose "${env_args[@]}" -p "${project_name}"', smoke)

    def test_smoke_uses_gateway_task_controls_and_owner_boundaries(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-interactive-shadow-compose.sh").read_text(encoding="utf-8")
        self.assertIn('request("POST", "http://gateway:8080/api/v1/agent/tasks"', smoke)
        self.assertIn('request("GET", `http://gateway:8080/api/v1/agent/tasks/${taskId}`', smoke)
        self.assertIn('request("POST", `http://gateway:8080/api/v1/agent/tasks/${taskId}/cancel`', smoke)
        self.assertIn('duplicate task start diverged', smoke)
        self.assertIn('foreign owner read was not rejected', smoke)
        self.assertIn('interactive task did not enter waiting_input', smoke)
        self.assertIn('lastTaskStatus = "unavailable"', smoke)
        self.assertIn('last status: ${lastTaskStatus}', smoke)
        self.assertIn('interactive task did not cancel', smoke)
        self.assertIn('interactive read task wrote', smoke)

    def test_model_stub_stays_inside_the_compose_project(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-interactive-shadow-smoke.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_STUB_FILE', overlay)
        self.assertIn('entrypoint: ["/bin/sh", "-ec"]', overlay)
        self.assertIn('node /app/model-stub.mjs & exec node dist/index.js', overlay)
        self.assertIn('DIPOLE_AGENT_KAFKA_GROUP_ID:', overlay)
        self.assertIn('DIPOLE_AGENT_MODEL_PROVIDER: openai_compatible', overlay)
        self.assertIn('DIPOLE_AGENT_MODEL_ROUTES:', overlay)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_SHADOW_TASK_QUEUE', overlay)


if __name__ == "__main__":
    unittest.main()
