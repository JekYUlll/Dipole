import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


class RecoveryDrillContractTest(unittest.TestCase):
    def test_drill_recovers_target_and_binds_post_load_report(self):
        script = (ROOT / "scripts/bench/recovery_drill.sh").read_text(encoding="utf-8")

        self.assertIn('TARGET_SERVICE="${TARGET_SERVICE:-dipole-node2}"', script)
        self.assertIn("dipole-node1|dipole-node2|dipole-node3", script)
        self.assertIn('trap recover_target EXIT', script)
        self.assertIn('compose stop "${TARGET_SERVICE}"', script)
        self.assertIn('compose start "${TARGET_SERVICE}"', script)
        self.assertIn("unavailable_observed_at", script)
        self.assertIn("ready_observed_at", script)
        self.assertIn("scripts/bench/run_bench.sh", script)
        self.assertIn("scripts/bench/recovery_report.py", script)
        self.assertLess(script.index('compose start "${TARGET_SERVICE}"'), script.index("scripts/bench/run_bench.sh"))
        self.assertNotIn("down --volumes", script)


if __name__ == "__main__":
    unittest.main()
