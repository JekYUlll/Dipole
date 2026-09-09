import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class RuntimeReadinessContractTest(unittest.TestCase):
    def test_smoke_uses_isolated_certificates_and_assignment_evidence(self):
        compose = (ROOT / "deploy/compose/docker-compose.microservices.yml").read_text(encoding="utf-8")
        smoke = (ROOT / "scripts/smoke-runtime-dependency-readiness.sh").read_text(
            encoding="utf-8"
        )

        self.assertNotIn("- ./certs/internal/", compose)
        self.assertIn("${DIPOLE_INTERNAL_CERT_DIR:-../../certs/internal}", compose)
        self.assertIn('INTERNAL_CERT_DIR="${cert_dir}"', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_API_KEY:=runtime-readiness-smoke-no-network', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_BASE_URL:=https://models.invalid/v1', smoke)
        self.assertIn('ports: !override []', smoke)
        self.assertIn('compose_files+=(-f "${ports_file}")', smoke)
        self.assertIn("assert_dependency_ready gateway kafka-assignment", smoke)
        self.assertIn("timeout --kill-after=2s 10s docker compose", smoke)


if __name__ == "__main__":
    unittest.main()
