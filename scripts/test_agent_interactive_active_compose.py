#!/usr/bin/env python3
"""Static contract for the explicit interactive active Agent overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class InteractiveAgentActiveComposeTest(unittest.TestCase):
    def test_overlay_exposes_task_control_and_read_only_definition_routes(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-interactive-active.yml").read_text(encoding="utf-8")
        self.assertIn("DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: interactive_active", overlay)
        self.assertIn('DIPOLE_AGENT_CONTROL_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_MESSAGE_WRITE_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_CONTROL_SECRET: ${DIPOLE_AGENT_CONTROL_SECRET:', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_ARTIFACT_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_MCP_ENABLED: "false"', overlay)

    def test_compose_gate_checks_the_effective_profile(self) -> None:
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn("agent-interactive-active.yml", checker)
        self.assertIn("interactive_active_config", checker)
        self.assertIn('DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED == "true"', checker)
        self.assertIn('DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED == "true"', checker)
        self.assertIn('DIPOLE_GATEWAY_AGENT_SUBSCRIPTION_ENABLED == "false"', checker)
        self.assertIn('DIPOLE_GATEWAY_AGENT_ARTIFACT_ENABLED == "false"', checker)

    def test_experience_profile_keeps_route_b_as_the_sole_inbound_responder(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-experience.yml").read_text(encoding="utf-8")
        for expected in (
            'DIPOLE_AI_DIRECT_REPLY_ENABLED: "false"',
            'DIPOLE_AI_GROUP_REPLY_ENABLED: "false"',
            'DIPOLE_AGENT_TEMPORAL_ENABLED: "true"',
            'DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: interactive_active',
            'DIPOLE_AGENT_INBOUND_INTERACTIVE_ENABLED: "true"',
            'DIPOLE_AGENT_INBOUND_GROUP_INTERACTIVE_ENABLED: "true"',
        ):
            self.assertIn(expected, overlay)

    def test_memory_smoke_is_explicit_and_isolates_retrieval(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-interactive-memory-smoke.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AGENT_MEMORY_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_MEMORY_PROFILE: "true"', overlay)
        self.assertIn('DIPOLE_AGENT_MODEL_MODE: ai_sdk', overlay)
        self.assertIn('DIPOLE_AGENT_RETRIEVAL_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_MEMORY_TASK_QUEUE', overlay)

    def test_memory_b1_smoke_is_an_explicit_isolated_profile(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-interactive-memory-b1-smoke.yml").read_text(encoding="utf-8")
        stub_overlay = (ROOT / "deploy/microservices/agent-interactive-memory-b1-stub.yml").read_text(encoding="utf-8")
        provider_overlay = (ROOT / "deploy/microservices/agent-interactive-memory-b1-provider.yml").read_text(encoding="utf-8")
        script = (ROOT / "scripts/smoke-agent-interactive-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_AI_DIRECT_REPLY_ENABLED: "false"', overlay)
        self.assertIn('DIPOLE_GATEWAY_AGENT_MEMORY_ENABLED: "true"', overlay)
        self.assertIn('DIPOLE_AI_AGENT_CANDIDATE_VERSION: ${DIPOLE_AGENT_CANDIDATE_VERSION:?DIPOLE_AGENT_CANDIDATE_VERSION is required}', overlay)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_MEMORY_B1_MODEL_STUB_FILE', stub_overlay)
        self.assertIn('DIPOLE_AGENT_MEMORY_B1_SMOKE:=0', script)
        self.assertIn('agent-interactive-memory-b1-smoke.yml', script)
        self.assertIn('DIPOLE_AGENT_MEMORY_B1_CANARY:=ORBIT-91', script)
        self.assertIn('DIPOLE_AGENT_MEMORY_B1_CANARY must be a 3-32 character uppercase token', script)
        self.assertIn('MEMORY-B1-CANARY: ${DIPOLE_AGENT_MEMORY_B1_CANARY}', script)
        self.assertIn('B1_MEMORY_RECALLED_ORBIT_91', script)
        self.assertIn('JOIN agent_model_runs AS runs ON runs.run_uuid = calls.run_uuid', script)
        self.assertIn('run_memory_b1()', script)
        self.assertIn('/api/v1/agent/memories/${memoryId}/revoke', script)
        self.assertIn('B1_MEMORY_MISSING', script)
        self.assertIn('DIPOLE_AGENT_MEMORY_B1_MODEL_SOURCE:=stub', script)
        self.assertIn('DIPOLE_AGENT_MEMORY_B1_MODEL_ENV_FILE is required for provider mode', script)
        self.assertIn('DIPOLE_AGENT_MEMORY_B1_SMOKE_RECEIPT_FILE', script)
        self.assertIn('DIPOLE_AGENT_MEMORY_B1_SYNTHETIC_EVAL_FILE', script)
        self.assertIn('dipole.agent.interactive-memory-b1-smoke-receipt.v1', script)
        self.assertIn('"firstTaskSha256":"${first_task_sha256}"', script)
        self.assertIn('"revokedTaskSha256":"${revoked_task_sha256}"', script)
        self.assertIn('"firstReplySha256":"${first_reply_sha256}"', script)
        self.assertIn('"firstResponseContainsCanary":${first_response_contains_canary}', script)
        self.assertIn('dipole.agent.memory-b1-synthetic-eval.v1', script)
        self.assertIn('"caseId":"recall-b1"', script)
        self.assertIn('"caseId":"revoke-b1"', script)
        self.assertIn('"revokedMemoryLineageCount":0', script)
        self.assertIn('agent-interactive-memory-b1-stub.yml', script)
        self.assertIn('agent-interactive-memory-b1-provider.yml', script)
        self.assertIn('modelSource":"${memory_b1_model_source}"', script)
        self.assertIn('DIPOLE_AGENT_INBOUND_INTERACTIVE_ENABLED: "true"', stub_overlay)
        self.assertIn('DIPOLE_AGENT_INBOUND_INTERACTIVE_ENABLED: "true"', provider_overlay)
        self.assertIn('DIPOLE_AGENT_MODEL_PROVIDER: openai_compatible', provider_overlay)

    def test_agent_runtime_can_persist_pre_model_memory_lineage(self) -> None:
        grants = (ROOT / "configs/mysql/agent-service-grants.dist.sql").read_text(encoding="utf-8")
        self.assertIn("GRANT SELECT, INSERT, UPDATE ON dipole.agent_memory_task_lineage TO 'dipole_agent'@'%';", grants)


if __name__ == "__main__":
    unittest.main()
