#!/usr/bin/env python3
"""Static safety contract for the isolated interactive-active Compose smoke."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class InteractiveAgentActiveComposeSmokeTest(unittest.TestCase):
    def test_smoke_is_isolated_and_requires_explicit_active_profiles(self) -> None:
        script = (ROOT / "scripts/smoke-agent-interactive-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn('project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-interactive-active-', script)
        self.assertIn('agent-temporal-read-shadow.yml', script)
        self.assertIn('agent-active.yml', script)
        self.assertIn('agent-interactive-active.yml', script)
        self.assertIn('DIPOLE_AGENT_INTERACTIVE_TASK_QUEUE:=dipole-agent-interactive-', script)
        self.assertIn('DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID:=dipole-agent-active-interactive-', script)
        self.assertIn('DIPOLE_AGENT_TEMPORAL_TASK_QUEUE:=${DIPOLE_AGENT_INTERACTIVE_TASK_QUEUE}', script)
        self.assertIn('DIPOLE_GATEWAY_BIND_ADDRESS:=127.0.0.1', script)
        self.assertIn('DIPOLE_MYSQL_AIO_COMPAT:=0', script)
        self.assertIn('remote-gpu-mysql-aio-compat.yml', script)
        self.assertIn('export INTERNAL_CERT_DIR="${DIPOLE_INTERNAL_CERT_DIR}"', script)
        self.assertIn('compose down --volumes --remove-orphans', script)
        self.assertIn('register_owner()', script)
        self.assertIn('http://gateway:8080/api/v1/auth/register', script)
        self.assertNotIn('http://core:8081/api/v1/auth/', script)
        self.assertGreaterEqual(script.count('http://gateway:8080/api/v1/auth/login'), 2)
        self.assertIn('owner_uuid=$(compose exec -T agent node', script)
        self.assertIn('verify_runtime_status()', script)
        self.assertIn('http://gateway:8080/api/v1/agent/status', script)
        self.assertIn('schemaVersion: "dipole.agent.runtime_status.v1"', script)
        self.assertIn('runtimeMode: "active"', script)
        self.assertIn('interactiveMessageWritesEnabled: true', script)
        self.assertIn('verify_runtime_status', script)
        self.assertIn('verify_definition_catalog()', script)
        self.assertIn('http://gateway:8080/api/v1/agent/definitions', script)
        self.assertIn('definition_uuid=$(verify_definition_catalog)', script)
        self.assertIn(': "${DIPOLE_AGENT_DEFINITION_ONLY:=0}"', script)
        self.assertIn("Agent Definition Compose smoke passed: project=%s", script)
        self.assertIn('if [[ "${DIPOLE_AGENT_DEFINITION_ONLY}" == "1" ]]; then', script)
        self.assertIn('"x-dipole-principal-user-id": ownerUuid', script)
        self.assertIn("INSERT IGNORE INTO users", script)
        self.assertIn('canonical_direct_conversation()', script)
        self.assertIn('conversation_key=$(canonical_direct_conversation "${owner_uuid}" "${agent_uuid}")', script)
        self.assertIn("JSON_CONTAINS(permissions_json, JSON_QUOTE('conversation.list'), '\\$')", script)
        self.assertIn("JSON_CONTAINS(permissions_json, JSON_QUOTE('conversation.read'), '\\$')", script)
        self.assertIn("JSON_CONTAINS(scopes_json, JSON_OBJECT('resource_type', 'conversation', 'resource_id', '*', 'actions', JSON_ARRAY('list')), '\\$')", script)
        self.assertIn("JSON_CONTAINS(scopes_json, JSON_OBJECT('resource_type', 'conversation', 'resource_id', '*', 'actions', JSON_ARRAY('read')), '\\$')", script)
        self.assertIn('expected_definition_record="${owner_uuid}"$\'\\tUAI000000000000000001\\t1\\t1\\t1\\t1\'', script)
        self.assertIn("'resource_id', '${conversation_key}'", script)
        self.assertIn("'${definition_uuid}', 2, 'dipole', '${owner_uuid}', '${agent_uuid}', 'active'", script)
        self.assertIn("'${grant_uuid}', 'dipole', 'dipole-agent', '${DIPOLE_AGENT_CANDIDATE_VERSION}', '${definition_uuid}', 2", script)

    def test_smoke_verifies_both_replay_outcomes_and_revokes_authority(self) -> None:
        script = (ROOT / "scripts/smoke-agent-interactive-active-compose.sh").read_text(encoding="utf-8")
        self.assertIn("wait_for_agent_ready()", script)
        self.assertIn("compose restart agent", script)
        self.assertIn('approved_approval=$(wait_for_approval "${approved_task}")\nrestart_agent_worker\nresolve_twice', script)
        self.assertIn('resolve_twice "${denied_task}" "${denied_approval}" denied', script)
        self.assertIn("[[ \"${denied_effects}\" == $'0\\t0' ]]", script)
        self.assertIn('resolve_twice "${approved_task}" "${approved_approval}" approved', script)
        self.assertIn("[[ \"${approved_effects}\" == $'1\\t1\\t1\\t1\\t2' ]]", script)
        self.assertIn("UPDATE agent_runtime_promotion_grants SET revoked_at = UTC_TIMESTAMP(3)", script)
        self.assertIn('DIPOLE_AGENT_MODEL_BASE_URL="https://models.invalid/v1"', script)
        self.assertIn('DIPOLE_AGENT_MODEL_ROUTES="compose-smoke/deterministic"', script)


if __name__ == "__main__":
    unittest.main()
