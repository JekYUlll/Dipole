import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class SyncCassandraPrimarySmokeTest(unittest.TestCase):
    def test_smoke_uses_ephemeral_certificates_and_no_network_model_config(self):
        smoke = (ROOT / "scripts/smoke-sync-cassandra-primary-compose.sh").read_text(
            encoding="utf-8"
        )

        self.assertIn('mktemp -d -t dipole-sync-cassandra-primary-certs.', smoke)
        self.assertIn('INTERNAL_CERT_DIR="${cert_dir}"', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_API_KEY:=sync-cassandra-primary-smoke-no-network', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_BASE_URL:=https://models.invalid/v1', smoke)
        self.assertIn('rm -rf "${cert_dir}"', smoke)


if __name__ == "__main__":
    unittest.main()
