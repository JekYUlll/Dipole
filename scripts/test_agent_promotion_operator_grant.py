#!/usr/bin/env python3
"""Exercise the fail-closed operator-grant operational interface."""

from pathlib import Path
import os
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts/manage-agent-promotion-operator-grant.sh"


class AgentPromotionOperatorGrantScriptTest(unittest.TestCase):
    def base_command(self, *extra: str) -> list[str]:
        return [
            "bash", str(SCRIPT), "grant",
            "--compose-project", "dipole-test",
            "--compose-file", "compose.yml",
            "--user", "operator-a",
            "--granted-by", "operator-b",
            "--ticket", "OPS-123",
            "--reason", "temporary smoke authority",
            "--roles", "propose,review",
            "--expires-at", "2030-09-05T12:00:00Z",
            *extra,
        ]

    def test_dry_run_does_not_require_docker_or_password(self) -> None:
        result = subprocess.run(self.base_command(), cwd=ROOT, text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Dry run only", result.stdout)

    def test_apply_uses_explicit_container_mysql_socket(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn("mysql --socket=/var/run/mysqld/mysqld.sock", source)

    def test_apply_streams_password_and_writes_grant_audit(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_path = Path(temp_dir)
            capture = temp_path / "capture"
            fake_docker = temp_path / "docker"
            fake_openssl = temp_path / "openssl"
            fake_docker.write_text(
                "#!/usr/bin/env bash\ncat > \"$CAPTURE\"\nprintf 'granted\\taudit-1\\n'\n",
                encoding="utf-8",
            )
            fake_openssl.write_text("#!/usr/bin/env bash\nprintf 'a%.0s' {1..64}\nprintf '\\n'\n", encoding="utf-8")
            fake_docker.chmod(0o755)
            fake_openssl.chmod(0o755)
            env = os.environ | {
                "PATH": f"{temp_path}:{os.environ['PATH']}",
                "CAPTURE": str(capture),
                "DIPOLE_AGENT_PROMOTION_MYSQL_ROOT_PASSWORD": "test-password",
            }
            result = subprocess.run(self.base_command("--apply"), cwd=ROOT, text=True, capture_output=True, env=env, check=False)
            self.assertEqual(result.returncode, 0, result.stderr)
            sent = capture.read_text(encoding="utf-8")
            self.assertTrue(sent.startswith("test-password\n"))
            self.assertIn("agent_runtime_promotion_operator_grant_audits", sent)
            self.assertIn("'granted'", sent)
            self.assertNotIn("test-password", result.stdout)

    def test_rejects_self_grant_and_revoke_roles(self) -> None:
        self_grant = self.base_command()
        self_grant[self_grant.index("operator-b")] = "operator-a"
        result = subprocess.run(self_grant, cwd=ROOT, text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode, 2)
        self.assertIn("must be different", result.stderr)

        revoke = self.base_command()
        revoke[2] = "revoke"
        result = subprocess.run(revoke, cwd=ROOT, text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode, 2)
        self.assertIn("does not accept", result.stderr)

    def test_rejects_invalid_or_expired_grant_expiry(self) -> None:
        invalid = self.base_command()
        invalid[invalid.index("2030-09-05T12:00:00Z")] = "2030-99-05T12:00:00Z"
        result = subprocess.run(invalid, cwd=ROOT, text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode, 2)
        self.assertIn("not a valid UTC timestamp", result.stderr)

        expired = self.base_command()
        expired[expired.index("2030-09-05T12:00:00Z")] = "2020-09-05T12:00:00Z"
        result = subprocess.run(expired, cwd=ROOT, text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode, 2)
        self.assertIn("must be in the future", result.stderr)


if __name__ == "__main__":
    unittest.main()
