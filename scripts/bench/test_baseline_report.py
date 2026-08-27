import json
import tempfile
import unittest
from pathlib import Path

from scripts.bench.baseline_report import build_report, evaluate_report, render_markdown


class BaselineReportTest(unittest.TestCase):
    def setUp(self):
        self.summary = {
            "metrics": {
                "msg_attempted_total": {"count": 125, "rate": 41.67},
                "msg_accepted_total": {"count": 120, "rate": 40},
                "msg_rejected_total": {"count": 5, "rate": 1.67},
                "msg_received_total": {"count": 116, "rate": 38.67},
                "msg_expected_receipts_total": {"count": 120, "rate": 40},
                "msg_delivery_rate": {"value": 1.0},
                "msg_e2e_latency_ms": {"avg": 18.2, "med": 14.0, "p(95)": 42.5, "p(99)": 75.1, "max": 91.0},
                "http_req_failed": {"value": 0.01},
            }
        }
        self.operations = {
            "schema_version": "dipole.performance.operations.v1",
            "run_id": "g0-20260827",
            "scenario": "mixed",
            "environment": {"git_commit": "abc123", "cpu": "test cpu", "topology": "dist"},
            "parameters": {
                "user_count": 20,
                "group_size": 20,
                "bench_script": "bench.js",
                "phone_prefix": "137",
                "sender_count": 10,
                "messages_per_sender": 5,
                "hot_group_warmup_messages": 0,
                "hot_group_member_count_threshold": 200,
                "hot_group_message_threshold": 50,
            },
            "storage": {
                "direct": {"messages": 100, "inbox_rows": 100},
                "group": {"messages": 20, "inbox_rows": 380},
            },
            "kafka_lag_samples": [0, 7, 3, 0],
        }

    def test_builds_normalized_report(self):
        report = build_report(self.summary, self.operations)

        self.assertEqual(report["schema_version"], "dipole.performance.baseline.v1")
        self.assertEqual(report["run_id"], "g0-20260827")
        self.assertEqual(report["workload"]["attempted"], 125)
        self.assertEqual(report["workload"]["accepted"], 120)
        self.assertEqual(report["workload"]["rejected"], 5)
        self.assertEqual(report["workload"]["persisted"], 120)
        self.assertEqual(report["workload"]["acceptance_rate"], 0.96)
        self.assertEqual(report["workload"]["persistence_rate"], 1.0)
        self.assertEqual(report["workload"]["throughput_per_second"], 40)
        self.assertEqual(report["delivery"]["rate"], 0.966667)
        self.assertEqual(report["parameters"]["group_size"], 20)
        self.assertEqual(report["latency_ms"]["p50"], 14.0)
        self.assertEqual(report["latency_ms"]["p95"], 42.5)
        self.assertEqual(report["storage"]["direct"]["inbox_write_amplification"], 1.0)
        self.assertEqual(report["storage"]["group"]["inbox_write_amplification"], 19.0)
        self.assertEqual(report["kafka"]["peak_lag"], 7)
        self.assertEqual(report["kafka"]["settled_lag"], 0)

    def test_missing_samples_remain_explicit(self):
        self.summary["metrics"].pop("msg_e2e_latency_ms")
        self.operations["storage"]["group"] = {"messages": 0, "inbox_rows": 0}
        self.operations["kafka_lag_samples"] = []

        report = build_report(self.summary, self.operations)

        self.assertIsNone(report["latency_ms"]["p95"])
        self.assertIsNone(report["storage"]["group"]["inbox_write_amplification"])
        self.assertIsNone(report["kafka"]["peak_lag"])
        self.assertIsNone(report["kafka"]["settled_lag"])

    def test_markdown_contains_scope_and_key_metrics(self):
        markdown = render_markdown(build_report(self.summary, self.operations))

        self.assertIn("`g0-20260827`", markdown)
        self.assertIn("P95 | 42.50 ms", markdown)
        self.assertIn("Group | 20 | 380 | 19.00", markdown)
        self.assertIn("Messages per sender | 5", markdown)
        self.assertIn("Hot-group thresholds | members=200, messages=50", markdown)
        self.assertIn("该报告只描述本次环境", markdown)

    def test_json_round_trip_is_stable(self):
        report = build_report(self.summary, self.operations)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "baseline.json"
            path.write_text(json.dumps(report, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")
            self.assertEqual(json.loads(path.read_text(encoding="utf-8")), report)

    def test_gate_accepts_complete_settled_run(self):
        report = build_report(self.summary, self.operations)
        report["workload"].update({"attempted": 120, "accepted": 120, "rejected": 0, "persisted": 120})

        self.assertEqual(evaluate_report(report, minimum_delivery_rate=0.95), [])

    def test_gate_reports_each_failed_invariant(self):
        report = build_report(self.summary, self.operations)
        report["workload"].update({"attempted": 10, "accepted": 9, "rejected": 1, "persisted": 8})
        report["delivery"]["rate"] = 0.8
        report["kafka"]["settled_lag"] = 2

        self.assertEqual(
            evaluate_report(report, minimum_delivery_rate=0.9),
            [
                "accepted 9 of 10 attempted messages",
                "observed 1 rejected messages",
                "persisted 8 of 9 accepted messages",
                "delivery rate 0.800000 is below 0.900000",
                "Kafka settled lag is 2",
            ],
        )


if __name__ == "__main__":
    unittest.main()
