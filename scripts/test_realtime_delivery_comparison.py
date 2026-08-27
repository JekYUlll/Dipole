import json
import tempfile
import unittest
from pathlib import Path

import realtime_delivery_comparison as comparison


GO_REVISION = "a" * 40
CPP_REVISION = "b" * 40


def baseline(count=2):
    return {
        "schema_version": "dipole.performance.baseline.v4",
        "run_id": "c2-comparison-direct-2",
        "scenario": "concurrent",
        "environment": {"git_commit": GO_REVISION},
        "workload": {
            "attempted": count,
            "accepted": count,
            "rejected": 0,
            "persisted": count,
            "received": count,
            "expected_receipts": count,
            "message_type": 0,
        },
        "delivery": {"rate": 1.0, "http_failure_rate": 0},
        "kafka": {"settled_lag": 0},
    }


def evidence(event_id, partition, offset, outcome="projected", **overrides):
    value = {
        "schema_version": "dipole.realtime.shadow-evidence.v3",
        "topic": "dipole.message.direct.created",
        "partition": partition,
        "offset": offset,
        "source_event_id": event_id,
        "batch_id": f"shadow:{event_id}:{partition}:{offset}",
        "message_type": 0,
        "item_count": 1,
        "outcome": outcome,
        "error_code": "" if outcome == "projected" else "node_transport",
        "node_batch_count": 1,
        "presence_observed": 1,
        "presence_eligible": 1,
        "presence_stale": 0,
        "presence_malformed": 0,
        "offline_item_count": 0,
        "transport_requested": 1,
        "transport_observed": 1 if outcome == "projected" else 0,
        "transport_duplicate": 0,
        "transport_rejected": 0,
        "transport_backpressured": 0,
    }
    value.update(overrides)
    return value


class RealtimeDeliveryComparisonTest(unittest.TestCase):
    def test_eligible_report_folds_deferred_retry_by_kafka_coordinate(self):
        records = [
            evidence("E1", 0, 0, "deferred", transport_backpressured=1),
            evidence("E1", 0, 0),
            evidence("E2", 1, 0),
        ]
        report = comparison.build_report(baseline(), records, GO_REVISION, CPP_REVISION)

        self.assertEqual("eligible", report["decision"])
        self.assertEqual([], report["issues"])
        self.assertEqual(2, report["comparison"]["unique_kafka_records"])
        self.assertEqual(1, report["comparison"]["deferred_attempts"])
        self.assertEqual(2, report["comparison"]["final_transport_observed"])
        self.assertEqual(0, report["comparison"]["final_transport_backpressured"])

    def test_blocks_final_backpressure_and_workload_count_drift(self):
        records = [
            evidence("E1", 0, 0),
            evidence("E2", 1, 0, "deferred", transport_backpressured=1),
        ]
        report = comparison.build_report(baseline(3), records, GO_REVISION, CPP_REVISION)

        self.assertEqual("blocked", report["decision"])
        self.assertIn("Go accepted messages must equal unique C++ Kafka records", report["issues"])
        self.assertIn("every C++ Kafka record must finish projected", report["issues"])

    def test_filters_setup_system_messages_by_baseline_message_type(self):
        records = [
            evidence("SYSTEM1", 0, 0, message_type=3, offline_item_count=1,
                     node_batch_count=0, transport_requested=0, transport_observed=0),
            evidence("SYSTEM2", 1, 0, message_type=3, offline_item_count=1,
                     node_batch_count=0, transport_requested=0, transport_observed=0),
            evidence("E1", 0, 1),
            evidence("E2", 1, 1),
        ]

        report = comparison.build_report(baseline(), records, GO_REVISION, CPP_REVISION)

        self.assertEqual("eligible", report["decision"])
        self.assertEqual({"message_type": 0}, report["workload_filter"])
        self.assertEqual(4, report["comparison"]["observed_kafka_records"])
        self.assertEqual(2, report["comparison"]["filtered_out_kafka_records"])
        self.assertEqual(2, report["comparison"]["unique_kafka_records"])

    def test_rejects_revision_and_coordinate_identity_drift(self):
        with self.assertRaisesRegex(ValueError, "Go revision"):
            comparison.build_report(baseline(), [evidence("E1", 0, 0), evidence("E2", 1, 0)], "c" * 40, CPP_REVISION)
        duplicate_event = [evidence("E1", 0, 0), evidence("E1", 1, 0)]
        with self.assertRaisesRegex(ValueError, "source event"):
            comparison.build_report(baseline(), duplicate_event, GO_REVISION, CPP_REVISION)

        group_baseline = baseline()
        group_baseline["scenario"] = "group_blast"
        with self.assertRaisesRegex(ValueError, "scenario"):
            comparison.build_report(group_baseline, [evidence("E1", 0, 0)], GO_REVISION, CPP_REVISION)

    def test_loads_ndjson_and_binds_input_hashes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            baseline_path = root / "baseline.json"
            evidence_path = root / "evidence.ndjson"
            baseline_path.write_text(json.dumps(baseline()), encoding="utf-8")
            evidence_path.write_text(
                "\n".join(json.dumps(record) for record in [evidence("E1", 0, 0), evidence("E2", 1, 0)]) + "\n",
                encoding="utf-8",
            )

            report = comparison.build_report_from_files(
                baseline_path, evidence_path, GO_REVISION, CPP_REVISION
            )

            self.assertEqual(64, len(report["inputs"]["go_baseline_sha256"]))
            self.assertEqual(64, len(report["inputs"]["cpp_evidence_sha256"]))
            self.assertEqual(GO_REVISION, report["candidates"]["go_revision"])
            self.assertEqual(CPP_REVISION, report["candidates"]["cpp_revision"])


if __name__ == "__main__":
    unittest.main()
