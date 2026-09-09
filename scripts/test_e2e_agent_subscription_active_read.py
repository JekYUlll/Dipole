#!/usr/bin/env python3
"""Lock public subscription E2E to reviewed grants by default."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts/e2e-agent-subscription-active-read.sh"


class AgentSubscriptionActiveReadE2ETest(unittest.TestCase):
    def test_reviewed_grant_is_default_and_fixture_is_explicit(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn('GRANT_MODE="${DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_GRANT_MODE:-reviewed}"', source)
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_ALLOW_FIXTURE_GRANT=1', source)
        self.assertIn('DIPOLE_AGENT_SUBSCRIPTION_ACTIVE_GRANT_UUID', source)
        self.assertIn('reviewed grant mode requires a 64-character', source)

    def test_reviewed_grant_must_be_active_and_definition_bound(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn("revoked_at IS NULL", source)
        self.assertIn("valid_from <= UTC_TIMESTAMP(3)", source)
        self.assertIn("expires_at > UTC_TIMESTAMP(3)", source)
        self.assertIn("candidate_version", source)
        self.assertIn("definition_uuid", source)
        self.assertIn("reviewed grant binding mismatch", source)

    def test_cleanup_only_revokes_a_fixture_grant(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn('if (( fixture_grant )) && [[ -n "${grant_uuid}" ]]', source)


if __name__ == "__main__":
    unittest.main()
