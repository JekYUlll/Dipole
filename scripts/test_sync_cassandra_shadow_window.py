#!/usr/bin/env python3
"""Static safety contract for the bounded public Sync shadow runner."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class SyncCassandraShadowWindowTest(unittest.TestCase):
    def test_window_is_explicit_bounded_and_restores_mysql_mode(self) -> None:
        script = (ROOT / "scripts/run-sync-cassandra-shadow-window.sh").read_text(encoding="utf-8")

        self.assertIn('DIPOLE_SYNC_SHADOW_WINDOW_CONFIRM=yes', script)
        self.assertIn('window_seconds < 30 || window_seconds > 900', script)
        self.assertIn('DIPOLE_SYNC_SHADOW_WINDOW_EXERCISE must name an executable absolute path', script)
        self.assertIn('DIPOLE_SYNC_SHADOW_WINDOW_OUTPUT_DIR must name a new absolute path', script)
        self.assertIn('compose_shadow_cmd up -d --no-deps sync', script)
        self.assertIn('compose_base_cmd up -d --no-deps sync', script)
        self.assertIn('trap restore_mysql_hydration EXIT INT TERM', script)
        self.assertIn('-mode shadow', script)
        self.assertNotIn('DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION=true', script)


if __name__ == "__main__":
    unittest.main()
