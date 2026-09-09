#!/usr/bin/env python3
"""Guard the shared reviewed-grant helper's fail-closed contract."""

from pathlib import Path
import subprocess
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts/manage-agent-subscription-reviewed-grant.sh"


class ReviewedGrantHelperTest(unittest.TestCase):
    def test_shell_syntax(self) -> None:
        subprocess.run(["bash", "-n", str(SCRIPT)], check=True)

    def test_refuses_execution_without_explicit_opt_in(self) -> None:
        result = subprocess.run([str(SCRIPT)], capture_output=True, text=True, check=False)
        self.assertEqual(result.returncode, 2)
        self.assertIn("ALLOW_REVIEWED_GRANT_HELPER=1", result.stderr)

    def test_requires_explicit_opt_in_and_secure_locations(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn("DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_ALLOW_REVIEWED_GRANT_HELPER", source)
        self.assertIn("DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_CONFIG", source)
        self.assertIn("DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_REVIEWED_GRANT_STATE_FILE", source)
        self.assertIn('[[ "$config" == /* && -r "$config" ]]', source)
        self.assertIn('[[ "$state_file" == /* ]]', source)
        self.assertNotIn("source .env", source)

    def test_uses_runtime_artifact_publication_and_gateway_review(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn("PromotionEvidencePublisher", source)
        self.assertIn("createAgentCapabilityRPC", source)
        self.assertIn("/api/v1/agent/runtime-promotions", source)
        self.assertIn("/review", source)
        self.assertIn("run-agent-promotion-window.sh\" open", source)
        self.assertIn("printf '%08d'", source)
        self.assertIn('require_id definition "$definition_uuid" 64', source)
        self.assertIn('require_id subscription "$subscription_uuid" 64', source)
        self.assertIn('docker compose --env-file "$env_file" -p "$project"', source)
        self.assertIn('exec -T mysql sh -ceu', source)
        self.assertNotIn('docker exec "${project}-mysql-1"', source)

    def test_cleanup_revokes_scoped_grant_roles_and_window(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn("REVIEWED_GRANT_ACTION:-grant", source)
        self.assertIn("SET revoked_at=COALESCE(revoked_at, UTC_TIMESTAMP(3))", source)
        self.assertIn('operator revoke "$proposer_uuid" "$owner_uuid"', source)
        self.assertIn('operator revoke "$reviewer_uuid" "$owner_uuid"', source)
        self.assertIn('run-agent-promotion-window.sh" close', source)
        self.assertIn("chmod", source, "state file creation must be owner-only")


if __name__ == "__main__":
    unittest.main()
