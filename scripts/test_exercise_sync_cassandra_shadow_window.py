#!/usr/bin/env python3
"""Static contract for the real Sync shadow-window exercise."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class ExerciseSyncCassandraShadowWindowTest(unittest.TestCase):
    def test_exercise_uses_message_then_read_only_sync_pages(self) -> None:
        script = (ROOT / "scripts/exercise-sync-cassandra-shadow-window.sh").read_text(encoding="utf-8")

        self.assertIn('type: "chat.send"', script)
        self.assertIn('agent_uuid="${AGENT_UUID:-UAI000000000000000001}"', script)
        self.assertIn("agent_replies", script)
        self.assertIn("user_sync_inbox", script)
        self.assertIn('/api/v1/sync?after_seq=0&limit=20', script)
        self.assertNotIn('/sync/checkpoint', script)
        self.assertNotIn('/sync/checkpoint', script)


if __name__ == "__main__":
    unittest.main()
