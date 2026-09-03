#!/usr/bin/env python3
"""Regression contract for the isolated Remote GPU Agent startup instructions."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]
DOCUMENT = ROOT / "docs/operations/REMOTE-DEV-DEPLOYMENT.md"


class RemoteDevDeploymentDocsTest(unittest.TestCase):
    def test_agent_candidate_generates_certificates_before_compose_render(self) -> None:
        text = DOCUMENT.read_text(encoding="utf-8")

        certificate_step = 'INTERNAL_CERT_DIR="${DIPOLE_INTERNAL_CERT_DIR}" scripts/generate-internal-certs.sh'
        compose_step = 'docker compose --env-file .env -p "${DIPOLE_PROJECT}"'

        self.assertIn('export DIPOLE_INTERNAL_CERT_DIR="${DIPOLE_ROOT}/.runtime/${DIPOLE_PROJECT}/internal-certs"', text)
        self.assertIn(certificate_step, text)
        self.assertIn(compose_step, text)
        self.assertGreater(text.index(compose_step, text.index(certificate_step)), text.index(certificate_step))


if __name__ == "__main__":
    unittest.main()
