#!/usr/bin/env python3
"""Static contract for the Cassandra Sync shadow hydration overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class CassandraSyncShadowComposeTest(unittest.TestCase):
    def test_overlay_only_changes_sync_to_shadow_hydration(self) -> None:
        overlay = (ROOT / "deploy/microservices/cassandra-sync-shadow.yml").read_text(
            encoding="utf-8"
        )

        self.assertIn("sync:", overlay)
        self.assertIn("cassandra-init:", overlay)
        self.assertIn('DIPOLE_CASSANDRA_ENABLED: "true"', overlay)
        self.assertIn("DIPOLE_CASSANDRA_HOSTS: cassandra:9042", overlay)
        self.assertIn("DIPOLE_CASSANDRA_KEYSPACE: dipole_message_shadow", overlay)
        self.assertIn('DIPOLE_SYNC_CASSANDRA_SHADOW_HYDRATION: "true"', overlay)
        self.assertIn('DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION: "false"', overlay)
        self.assertNotIn("gateway:", overlay)
        self.assertNotIn("message:", overlay)
        self.assertNotIn("core:", overlay)

    def test_smoke_uses_ephemeral_certificates_and_checks_shadow_mode(self) -> None:
        smoke = (ROOT / "scripts/smoke-sync-cassandra-shadow-compose.sh").read_text(
            encoding="utf-8"
        )

        self.assertIn('mktemp -d -t dipole-sync-cassandra-shadow-certs.', smoke)
        self.assertIn('INTERNAL_CERT_DIR="${cert_dir}"', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_API_KEY:=sync-cassandra-shadow-smoke-no-network', smoke)
        self.assertIn('DIPOLE_SYNC_IMAGE:=dipole-sync:cassandra-shadow-${revision}', smoke)
        self.assertIn('DIPOLE_MIGRATE_IMAGE:=dipole-migrate:cassandra-shadow-${revision}', smoke)
        self.assertIn('DIPOLE_CASSANDRA_PROJECTOR_IMAGE:=dipole-cassandra-projector:cassandra-shadow-${revision}', smoke)
        self.assertIn('build_service_image dipole-sync ./cmd/services/sync "${DIPOLE_SYNC_IMAGE}"', smoke)
        self.assertIn('build_service_image dipole-migrate ./cmd/tools/migrate "${DIPOLE_MIGRATE_IMAGE}"', smoke)
        self.assertIn('build_service_image dipole-cassandra-projector ./cmd/tools/cassandra-projector "${DIPOLE_CASSANDRA_PROJECTOR_IMAGE}"', smoke)
        self.assertIn('DIPOLE_SYNC_SMOKE_BUILD_IMAGE:-1', smoke)
        self.assertIn('test "${sync_env}" = "true:true:false"', smoke)
        self.assertIn('rm -rf "${cert_dir}"', smoke)


if __name__ == "__main__":
    unittest.main()
