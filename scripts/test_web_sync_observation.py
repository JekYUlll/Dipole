import json
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

from scripts.web_sync_observation import (
    build_final_evidence,
    build_session,
    build_status,
    verify_candidate,
    write_immutable_json,
)


class FakePrometheus:
    def __init__(self, values):
        self.values = values

    def query(self, expression, captured_at):
        if expression not in self.values:
            raise AssertionError(f"unexpected query: {expression}")
        value = self.values[expression]
        return {
            "status": "success",
            "data": {
                "resultType": "vector",
                "result": [] if value is None else [{"metric": {}, "value": [captured_at.timestamp(), str(value)]}],
            },
        }


START = datetime(2026, 8, 28, 0, 0, tzinfo=timezone.utc)
COMMIT = "a" * 40


def clean_start_values():
    return {
        'count(dipole_web_sync_comparison_total{scope="incoming_direct",outcome="match"})': 1,
        'sum(kafka_consumergroup_lag{consumergroup="dipole-sync-consumer"})': 0,
        "dipole:web_sync_shadow:window_complete": 0,
        'count(ALERTS{alertname=~"DipoleSyncProjector(Lag|Retry|DeadLetter)|DipoleWebSync(ShadowDivergence|ShadowOverflow|StorageFull|ClientErrors)",alertstate="firing"})': None,
    }


def clean_final_values():
    return {
        "dipole:web_sync_shadow:matches_24h": 120,
        "dipole:web_sync_shadow:terminal_differences_24h": 0,
        "dipole:web_sync_shadow:overflows_24h": 0,
        "dipole:web_sync_shadow:window_complete": 1,
        "dipole:web_sync_shadow:promotion_ready": 1,
        'count(ALERTS{alertname=~"DipoleSyncProjector(Lag|Retry|DeadLetter)|DipoleWebSync(ShadowDivergence|ShadowOverflow|StorageFull|ClientErrors)",alertstate="firing"})': None,
    }


def clean_archive():
    return {
        "uri": "s3://dipole-evidence/web-sync/session.json",
        "object_version": "version-001",
        "etag": "a" * 32,
        "retention_until": "2026-09-30T00:00:00Z",
    }


class WebSyncObservationTest(unittest.TestCase):
    def test_observability_smoke_bounds_isolated_compose_startup(self):
        smoke = Path("scripts/smoke-web-sync-observability.sh").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_WEB_SYNC_OBSERVABILITY_STARTUP_TIMEOUT_SECONDS', smoke)
        self.assertIn('command -v timeout', smoke)
        self.assertIn('timeout --preserve-status "${startup_timeout_seconds}s" docker compose', smoke)
        self.assertIn('startup timeout must be between 30 and 1800 seconds', smoke)

    def test_language_neutral_contracts_are_strict_and_versioned(self):
        root = Path("contracts/web-sync-observation/v1")
        session_schema = json.loads((root / "session.schema.json").read_text(encoding="utf-8"))
        evidence_schema = json.loads((root / "evidence.schema.json").read_text(encoding="utf-8"))
        self.assertFalse(session_schema["additionalProperties"])
        self.assertFalse(evidence_schema["additionalProperties"])
        self.assertEqual(session_schema["properties"]["schema_version"]["const"], "dipole.web-sync.observation-session.v1")
        self.assertEqual(evidence_schema["properties"]["schema_version"]["const"], "dipole.web-sync.observation-evidence.v1")
        self.assertEqual(evidence_schema["properties"]["duration_seconds"]["minimum"], 86400)

    def test_session_binds_candidate_commit_bundle_and_preflight(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "app.js"
            bundle.write_bytes(b"candidate bundle")
            first = build_session("web-sync-v1", COMMIT, bundle, "http://prometheus:9090", START, FakePrometheus(clean_start_values()))
            second = build_session("web-sync-v1", COMMIT, bundle, "http://prometheus:9090", START, FakePrometheus(clean_start_values()))
        self.assertEqual(first, second)
        self.assertEqual(first["schema_version"], "dipole.web-sync.observation-session.v1")
        self.assertRegex(first["session_id"], r"^web-sync:[a-f0-9]{64}$")
        self.assertEqual(first["candidate"]["git_commit"], COMMIT)
        self.assertRegex(first["candidate"]["bundle_sha256"], r"^[a-f0-9]{64}$")
        self.assertEqual(first["minimum_end_at"], "2026-08-29T00:00:00.000Z")

    def test_session_rejects_prometheus_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "app.js"
            bundle.write_bytes(b"candidate bundle")
            with self.assertRaises(ValueError):
                build_session("web-sync-v1", COMMIT, bundle, "https://user:secret@prometheus.example/query", START, FakePrometheus(clean_start_values()))

    def test_session_rejects_missing_metric_or_dirty_projector(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "app.js"
            bundle.write_bytes(b"candidate bundle")
            for mutate in (
                lambda values: values.update({'count(dipole_web_sync_comparison_total{scope="incoming_direct",outcome="match"})': None}),
                lambda values: values.update({'sum(kafka_consumergroup_lag{consumergroup="dipole-sync-consumer"})': 1}),
            ):
                values = clean_start_values()
                mutate(values)
                with self.assertRaises(ValueError):
                    build_session("web-sync-v1", COMMIT, bundle, "http://prometheus:9090", START, FakePrometheus(values))

    def test_finalize_rejects_short_window(self):
        session = self._session()
        with self.assertRaises(ValueError):
            build_final_evidence(session, START + timedelta(hours=23, minutes=59), FakePrometheus(clean_final_values()))

    def test_session_rejects_future_start(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "app.js"
            bundle.write_bytes(b"candidate bundle")
            with self.assertRaisesRegex(ValueError, "session start cannot be more than 5 minutes"):
                build_session(
                    "web-sync-v1",
                    COMMIT,
                    bundle,
                    "http://prometheus:9090",
                    START,
                    FakePrometheus(clean_start_values()),
                    now=START - timedelta(minutes=6),
                )

    def test_status_and_finalize_reject_future_capture(self):
        session = self._session()
        future = START + timedelta(hours=24, minutes=6)
        with self.assertRaisesRegex(ValueError, "status capture cannot be more than 5 minutes"):
            build_status(session, future, FakePrometheus(clean_final_values()), now=START + timedelta(hours=24))
        with self.assertRaisesRegex(ValueError, "observation end cannot be more than 5 minutes"):
            build_final_evidence(session, future, FakePrometheus(clean_final_values()), now=START + timedelta(hours=24))

    def test_status_rejects_capture_before_session_start(self):
        session = self._session()
        with self.assertRaisesRegex(ValueError, "cannot precede"):
            build_status(session, START - timedelta(seconds=1), FakePrometheus(clean_final_values()))

    def test_candidate_verification_rejects_commit_or_bundle_drift(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "app.js"
            bundle.write_bytes(b"candidate bundle")
            session = build_session("web-sync-v1", COMMIT, bundle, "http://prometheus:9090", START, FakePrometheus(clean_start_values()))
            verify_candidate(session, COMMIT, bundle)
            with self.assertRaises(ValueError):
                verify_candidate(session, "b" * 40, bundle)
            bundle.write_bytes(b"changed bundle")
            with self.assertRaises(ValueError):
                verify_candidate(session, COMMIT, bundle)

    def test_session_binds_every_file_in_a_deployed_bundle_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "webapp"
            assets = bundle / "assets"
            assets.mkdir(parents=True)
            (bundle / "index.html").write_text("<script src='/assets/app.js'></script>", encoding="utf-8")
            (assets / "app.js").write_bytes(b"first bundle")
            session = build_session("web-sync-v1", COMMIT, bundle, "http://prometheus:9090", START, FakePrometheus(clean_start_values()))
            verify_candidate(session, COMMIT, bundle)

            (assets / "app.js").write_bytes(b"changed bundle")
            with self.assertRaises(ValueError):
                verify_candidate(session, COMMIT, bundle)

            (assets / "app.js").write_bytes(b"first bundle")
            (assets / "late.css").write_bytes(b"body{}")
            with self.assertRaises(ValueError):
                verify_candidate(session, COMMIT, bundle)

    def test_session_rejects_empty_or_symlinked_bundle_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "webapp"
            bundle.mkdir()
            with self.assertRaisesRegex(ValueError, "cannot be empty"):
                build_session("web-sync-v1", COMMIT, bundle, "http://prometheus:9090", START, FakePrometheus(clean_start_values()))

            (bundle / "index.html").write_text("ok", encoding="utf-8")
            (bundle / "linked.js").symlink_to(bundle / "index.html")
            with self.assertRaisesRegex(ValueError, "symbolic links"):
                build_session("web-sync-v1", COMMIT, bundle, "http://prometheus:9090", START, FakePrometheus(clean_start_values()))

    def test_finalize_archives_eligible_clean_window(self):
        session = self._session()
        evidence = build_final_evidence(session, START + timedelta(hours=24), FakePrometheus(clean_final_values()), archive=clean_archive())
        self.assertEqual(evidence["schema_version"], "dipole.web-sync.observation-evidence.v1")
        self.assertEqual(evidence["decision"], "eligible")
        self.assertEqual(evidence["issues"], [])
        self.assertRegex(evidence["snapshot_sha256"], r"^[a-f0-9]{64}$")
        self.assertEqual(evidence["archive"]["object_version"], "version-001")
        self.assertRegex(evidence["evidence_sha256"], r"^[a-f0-9]{64}$")

    def test_finalize_archives_blocked_window_without_weakening_thresholds(self):
        session = self._session()
        values = clean_final_values()
        values["dipole:web_sync_shadow:terminal_differences_24h"] = 1
        values["dipole:web_sync_shadow:promotion_ready"] = 0
        evidence = build_final_evidence(session, START + timedelta(hours=24), FakePrometheus(values), archive=clean_archive())
        self.assertEqual(evidence["decision"], "blocked")
        self.assertIn("terminal differences must be zero", evidence["issues"])

    def test_finalize_blocks_without_archive_receipt(self):
        session = self._session()
        evidence = build_final_evidence(session, START + timedelta(hours=24), FakePrometheus(clean_final_values()))
        self.assertEqual(evidence["decision"], "blocked")
        self.assertIn("evidence archive receipt is required", evidence["issues"])

    def test_archive_receipt_rejects_expired_or_non_opaque_metadata(self):
        session = self._session()
        for archive in (
            {**clean_archive(), "retention_until": "2026-08-29T00:00:00Z"},
            {**clean_archive(), "uri": "https://example.test/evidence"},
        ):
            with self.assertRaises(ValueError):
                build_final_evidence(session, START + timedelta(hours=24), FakePrometheus(clean_final_values()), archive=archive)

    def test_evidence_write_is_immutable(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "evidence.json"
            write_immutable_json(output, {"value": 1})
            self.assertEqual(json.loads(output.read_text(encoding="utf-8")), {"value": 1})
            with self.assertRaises(FileExistsError):
                write_immutable_json(output, {"value": 2})

    def _session(self):
        with tempfile.TemporaryDirectory() as directory:
            bundle = Path(directory) / "app.js"
            bundle.write_bytes(b"candidate bundle")
            return build_session("web-sync-v1", COMMIT, bundle, "http://prometheus:9090", START, FakePrometheus(clean_start_values()))


if __name__ == "__main__":
    unittest.main()
