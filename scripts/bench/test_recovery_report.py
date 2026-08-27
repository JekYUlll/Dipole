import hashlib
import json
import tempfile
import unittest
from pathlib import Path

from scripts.bench.recovery_report import build_report, main


REVISION = "a" * 40
IMAGE_ID = "sha256:" + "b" * 64
CONTAINER_ID = "c" * 64


def evidence():
    return {
        "schema_version": "dipole.performance.recovery-evidence.v1",
        "run_id": "c1-node2-recovery",
        "target_service": "dipole-node2",
        "expected_revision": REVISION,
        "fault": {"action": "stop_start"},
        "timeline": {
            "fault_started_at": "2026-08-28T08:00:00.000Z",
            "unavailable_observed_at": "2026-08-28T08:00:00.250Z",
            "start_requested_at": "2026-08-28T08:00:01.000Z",
            "ready_observed_at": "2026-08-28T08:00:02.750Z",
        },
        "before": {
            "container_id": CONTAINER_ID,
            "image_id": IMAGE_ID,
            "revision": REVISION,
            "source_dirty": False,
            "pid": 101,
        },
        "after": {
            "container_id": CONTAINER_ID,
            "image_id": IMAGE_ID,
            "revision": REVISION,
            "source_dirty": False,
            "pid": 202,
        },
    }


def baseline():
    services = {
        name: {
            "container_id": value,
            "image_id": IMAGE_ID,
            "revision": REVISION,
            "created": "2026-08-28T07:30:00Z",
            "source_dirty": False,
        }
        for name, value in {
            "dipole-node1": "1" * 64,
            "dipole-node2": CONTAINER_ID,
            "dipole-node3": "3" * 64,
        }.items()
    }
    return {
        "schema_version": "dipole.performance.baseline.v4",
        "environment": {"git_commit": REVISION},
        "workload": {
            "attempted": 40,
            "accepted": 40,
            "persisted": 40,
            "received": 40,
            "expected_receipts": 40,
            "acceptance_rate": 1.0,
            "persistence_rate": 1.0,
        },
        "delivery": {"rate": 1.0, "http_failure_rate": 0.0},
        "kafka": {"samples": [0, 12, 0], "peak_lag": 12, "settled_lag": 0},
        "process_resources": {
            "available": True,
            "services": {"dipole-node2": {"pid": 202}},
        },
        "runtime_provenance": {
            "available": True,
            "expected_revision": REVISION,
            "source_aligned": True,
            "services": services,
        },
    }


def baseline_bytes(value):
    return (json.dumps(value, sort_keys=True) + "\n").encode()


class RecoveryReportTest(unittest.TestCase):
    def test_builds_recovery_report_bound_to_post_load(self):
        raw_baseline = baseline_bytes(baseline())
        report = build_report(evidence(), json.loads(raw_baseline), hashlib.sha256(raw_baseline).hexdigest())

        self.assertEqual(report["schema_version"], "dipole.performance.recovery-report.v1")
        self.assertTrue(report["passed"])
        self.assertEqual(report["recovery"]["unavailable_to_ready_ms"], 2500)
        self.assertEqual(report["recovery"]["restart_to_ready_ms"], 1750)
        self.assertEqual(report["before"]["pid"], 101)
        self.assertEqual(report["after"]["pid"], 202)
        self.assertEqual(report["post_load"]["attempted"], 40)
        self.assertEqual(report["post_load"]["settled_kafka_lag"], 0)

    def test_rejects_unobserved_or_misaligned_recovery(self):
        cases = []

        same_pid = evidence()
        same_pid["after"]["pid"] = same_pid["before"]["pid"]
        cases.append(same_pid)

        dirty = evidence()
        dirty["after"]["source_dirty"] = True
        cases.append(dirty)

        wrong_image = evidence()
        wrong_image["after"]["image_id"] = "sha256:" + "d" * 64
        cases.append(wrong_image)

        wrong_order = evidence()
        wrong_order["timeline"]["ready_observed_at"] = "2026-08-28T07:59:59Z"
        cases.append(wrong_order)

        for candidate in cases:
            with self.subTest(candidate=candidate):
                with self.assertRaises(ValueError):
                    build_report(candidate, baseline(), "e" * 64)

    def test_rejects_post_load_that_did_not_fully_recover(self):
        cases = []

        incomplete = baseline()
        incomplete["workload"]["received"] = 39
        cases.append(incomplete)

        lagged = baseline()
        lagged["kafka"]["settled_lag"] = 1
        cases.append(lagged)

        stale_pid = baseline()
        stale_pid["process_resources"]["services"]["dipole-node2"]["pid"] = 101
        cases.append(stale_pid)

        drifted = baseline()
        drifted["runtime_provenance"]["services"]["dipole-node2"]["image_id"] = "sha256:" + "d" * 64
        cases.append(drifted)

        for candidate in cases:
            with self.subTest(candidate=candidate):
                with self.assertRaises(ValueError):
                    build_report(evidence(), candidate, "e" * 64)

    def test_cli_hashes_exact_baseline_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            evidence_path = root / "evidence.json"
            baseline_path = root / "baseline.json"
            output_path = root / "report.json"
            evidence_path.write_text(json.dumps(evidence()), encoding="utf-8")
            raw_baseline = baseline_bytes(baseline())
            baseline_path.write_bytes(raw_baseline)

            status = main([
                "--evidence", str(evidence_path),
                "--baseline", str(baseline_path),
                "--output", str(output_path),
            ])

            self.assertEqual(status, 0)
            report = json.loads(output_path.read_text(encoding="utf-8"))
            self.assertEqual(report["post_load"]["baseline_sha256"], hashlib.sha256(raw_baseline).hexdigest())


if __name__ == "__main__":
    unittest.main()
