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


if __name__ == "__main__":
    unittest.main()
