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
        self.assertIn('build_timeout_seconds="${DIPOLE_AGENT_INTERACTIVE_BUILD_TIMEOUT_SECONDS:-900}"', smoke)
        self.assertIn("DIPOLE_AGENT_INTERACTIVE_BUILD_TIMEOUT_SECONDS must be between 120 and 3600", smoke)
        self.assertIn('timeout --preserve-status "${build_timeout_seconds}"', smoke)
        self.assertIn('DIPOLE_MICROSERVICE_IMAGE_SERVICES="migrate,core,gateway,message,sync,agent"', smoke)
        self.assertIn('remote-gpu-mysql-aio-compat.yml', smoke)
        self.assertIn('agent-interactive-shadow-smoke.yml', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_BASE_URL="http://127.0.0.1:8089/v1"', smoke)
        self.assertIn('compose down --volumes --remove-orphans', smoke)

    def test_smoke_receipt_is_opt_in_low_sensitivity_and_atomic(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-interactive-shadow-compose.sh").read_text(encoding="utf-8")
        self.assertIn('receipt_file="${DIPOLE_AGENT_INTERACTIVE_SMOKE_RECEIPT_FILE:-}"', smoke)
        self.assertIn('must be a new absolute path in an existing directory', smoke)
        self.assertIn('"taskSha256":"${task_sha256}"', smoke)
        self.assertIn('"runtimeRevision":"${runtime_revision}"', smoke)
        self.assertIn("openssl dgst -sha256 -r", smoke)
        self.assertIn('ln "${receipt_temp}" "${receipt_file}"', smoke)
        self.assertNotIn('"taskId":"${task_uuid}"', smoke)
        self.assertNotIn('"ownerUuid":"${owner_uuid}"', smoke)

    def test_active_read_profile_stays_read_only_and_uses_a_revocable_grant(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-interactive-shadow-compose.sh").read_text(encoding="utf-8")
        overlay = (ROOT / "deploy/microservices/agent-interactive-read-active.yml").read_text(encoding="utf-8")
        self.assertIn('execution_profile="${DIPOLE_AGENT_INTERACTIVE_READ_PROFILE:-shadow}"', smoke)
        self.assertIn('agent-interactive-read-active.yml', smoke)
        self.assertIn('agent_runtime_promotion_grants', smoke)
        self.assertIn('WHERE grant_uuid = \'${grant_uuid}\'', smoke)
        self.assertIn('DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: read_active', overlay)
        self.assertIn('DIPOLE_AGENT_MCP_SERVER_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_AGENT_MEMORY_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_AGENT_RETRIEVAL_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED: "true"', overlay)

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
        self.assertIn('duplicate task start diverged', smoke)
        self.assertIn('foreign owner read was not rejected', smoke)
        self.assertIn('interactive task did not complete', smoke)
        self.assertIn('lastTaskStatus = "unavailable"', smoke)
        self.assertIn('last status: ${lastTaskStatus}', smoke)
        self.assertIn("agent_shadow_steps", smoke)
        self.assertIn('interactive read task wrote', smoke)
        self.assertIn('conversationId":"$discovered.previous', smoke)
        self.assertIn('read the available Agent conversation', smoke)

    def test_model_stub_stays_inside_the_compose_project(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-interactive-shadow-smoke.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_SHADOW_MODEL_STUB_FILE', overlay)
        self.assertIn('entrypoint: ["/bin/sh", "-ec"]', overlay)
        self.assertIn('node /app/model-stub.mjs & exec node dist/index.js', overlay)
        self.assertIn('DIPOLE_AGENT_KAFKA_GROUP_ID:', overlay)
        self.assertIn('DIPOLE_AGENT_MODEL_PROVIDER: openai_compatible', overlay)
        self.assertIn('DIPOLE_AGENT_MODEL_ROUTES:', overlay)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_SHADOW_TASK_QUEUE', overlay)

    def test_agent_image_keeps_production_dependencies_out_of_the_build_stage(self) -> None:
        dockerfile = (ROOT / "services/agent-runtime/Dockerfile").read_text(encoding="utf-8")
        self.assertIn("FROM node:22-bookworm-slim AS production-dependencies", dockerfile)
        self.assertIn("RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund", dockerfile)
        self.assertIn("COPY --from=production-dependencies --chown=node:node /app/node_modules ./node_modules", dockerfile)
        self.assertNotIn("npm prune", dockerfile)


if __name__ == "__main__":
    unittest.main()
