import unittest
from pathlib import Path

from scripts.bench.kafka_lag import parse_total_lag


ROOT = Path(__file__).resolve().parents[2]


class KafkaLagTest(unittest.TestCase):
    def test_counts_numeric_and_uninitialized_offsets_conservatively(self):
        output = """
GROUP TOPIC PARTITION CURRENT-OFFSET LOG-END-OFFSET LAG CONSUMER-ID HOST CLIENT-ID
dipole-consumer dipole.message.direct.send_requested 0 4 7 3 member /host dipole
dipole-consumer dipole.message.direct.send_requested 1 - 8 - member /host dipole
other-consumer topic 0 0 100 100 member /host other
"""

        self.assertEqual(parse_total_lag(output, "dipole"), 11)

    def test_rejects_missing_or_unparseable_group_evidence(self):
        with self.assertRaises(ValueError):
            parse_total_lag("GROUP TOPIC PARTITION CURRENT-OFFSET LOG-END-OFFSET LAG\n", "dipole")
        with self.assertRaises(ValueError):
            parse_total_lag("dipole-consumer topic 0 bad 8 bad member host client\n", "dipole")

    def test_run_bench_uses_the_fail_closed_parser(self):
        script = (ROOT / "scripts/bench/run_bench.sh").read_text(encoding="utf-8")

        self.assertIn("scripts/bench/kafka_lag.py", script)
        self.assertNotIn("$6 ~ /^[0-9]+$/ { total += $6 }", script)


if __name__ == "__main__":
    unittest.main()
