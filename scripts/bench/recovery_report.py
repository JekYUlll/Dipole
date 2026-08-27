#!/usr/bin/env python3
import argparse
from datetime import datetime
import hashlib
import json
from pathlib import Path
import re


EVIDENCE_SCHEMA_VERSION = "dipole.performance.recovery-evidence.v1"
REPORT_SCHEMA_VERSION = "dipole.performance.recovery-report.v1"
REVISION_PATTERN = re.compile(r"^[0-9a-f]{40}$")
CONTAINER_ID_PATTERN = re.compile(r"^[0-9a-f]{64}$")
IMAGE_ID_PATTERN = re.compile(r"^sha256:[0-9a-f]{64}$")
SHA256_PATTERN = re.compile(r"^[0-9a-f]{64}$")
EVIDENCE_FIELDS = {
    "schema_version",
    "run_id",
    "target_service",
    "expected_revision",
    "fault",
    "timeline",
    "before",
    "after",
}
SNAPSHOT_FIELDS = {
    "container_id",
    "image_id",
    "revision",
    "source_dirty",
    "pid",
}
TIMELINE_FIELDS = {
    "fault_started_at",
    "unavailable_observed_at",
    "start_requested_at",
    "ready_observed_at",
}


def _timestamp(value, field):
    if not isinstance(value, str) or not value:
        raise ValueError(f"{field} must be an RFC3339 timestamp")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as error:
        raise ValueError(f"{field} must be an RFC3339 timestamp") from error
    if parsed.tzinfo is None:
        raise ValueError(f"{field} must include an RFC3339 timezone")
    return parsed


def _positive_int(value, field):
    if isinstance(value, bool) or not isinstance(value, int) or value <= 0:
        raise ValueError(f"{field} must be a positive integer")
    return value


def _nonnegative_int(value, field):
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        raise ValueError(f"{field} must be a non-negative integer")
    return value


def _snapshot(value, field, expected_revision):
    if not isinstance(value, dict) or set(value) != SNAPSHOT_FIELDS:
        raise ValueError(f"{field} fields do not match recovery evidence v1")
    container_id = value["container_id"]
    image_id = value["image_id"]
    revision = value["revision"]
    if not isinstance(container_id, str) or not CONTAINER_ID_PATTERN.fullmatch(container_id):
        raise ValueError(f"{field}.container_id must be a full Docker container ID")
    if not isinstance(image_id, str) or not IMAGE_ID_PATTERN.fullmatch(image_id):
        raise ValueError(f"{field}.image_id must be a sha256 image ID")
    if not isinstance(revision, str) or not REVISION_PATTERN.fullmatch(revision):
        raise ValueError(f"{field}.revision must be a full lowercase Git revision")
    if revision != expected_revision:
        raise ValueError(f"{field}.revision does not match expected_revision")
    if value["source_dirty"] is not False:
        raise ValueError(f"{field}.source_dirty must be false")
    return {
        "container_id": container_id,
        "image_id": image_id,
        "revision": revision,
        "source_dirty": False,
        "pid": _positive_int(value["pid"], f"{field}.pid"),
    }


def _post_load(baseline, target_service, expected_revision, after, baseline_sha256):
    if not isinstance(baseline_sha256, str) or not SHA256_PATTERN.fullmatch(baseline_sha256):
        raise ValueError("baseline_sha256 must be a lowercase SHA-256")
    if not isinstance(baseline, dict) or baseline.get("schema_version") != "dipole.performance.baseline.v4":
        raise ValueError("post-load baseline must use baseline v4")
    environment = baseline.get("environment")
    if not isinstance(environment, dict) or environment.get("git_commit") != expected_revision:
        raise ValueError("post-load baseline revision does not match recovery evidence")

    workload = baseline.get("workload")
    delivery = baseline.get("delivery")
    kafka = baseline.get("kafka")
    if not all(isinstance(value, dict) for value in (workload, delivery, kafka)):
        raise ValueError("post-load baseline is missing workload, delivery, or kafka evidence")
    attempted = _positive_int(workload.get("attempted"), "post_load.attempted")
    accepted = _nonnegative_int(workload.get("accepted"), "post_load.accepted")
    persisted = _nonnegative_int(workload.get("persisted"), "post_load.persisted")
    received = _nonnegative_int(workload.get("received"), "post_load.received")
    expected_receipts = _positive_int(
        workload.get("expected_receipts"), "post_load.expected_receipts"
    )
    if not (
        accepted == attempted
        and persisted == attempted
        and received == expected_receipts
        and workload.get("acceptance_rate") == 1.0
        and workload.get("persistence_rate") == 1.0
        and delivery.get("rate") == 1.0
        and delivery.get("http_failure_rate") == 0.0
    ):
        raise ValueError("post-load baseline did not fully accept, persist, and deliver")
    settled_lag = _nonnegative_int(kafka.get("settled_lag"), "post_load.settled_kafka_lag")
    if settled_lag != 0:
        raise ValueError("post-load Kafka lag did not settle to zero")
    peak_lag = _nonnegative_int(kafka.get("peak_lag"), "post_load.peak_kafka_lag")

    resources = baseline.get("process_resources")
    if not isinstance(resources, dict) or resources.get("available") is not True:
        raise ValueError("post-load process resources are unavailable")
    resource_services = resources.get("services")
    target_resources = resource_services.get(target_service) if isinstance(resource_services, dict) else None
    if not isinstance(target_resources, dict) or target_resources.get("pid") != after["pid"]:
        raise ValueError("post-load target PID does not match recovery evidence")

    provenance = baseline.get("runtime_provenance")
    if not isinstance(provenance, dict) or provenance.get("available") is not True:
        raise ValueError("post-load runtime provenance is unavailable")
    if provenance.get("source_aligned") is not True or provenance.get("expected_revision") != expected_revision:
        raise ValueError("post-load runtime provenance is not source aligned")
    services = provenance.get("services")
    target = services.get(target_service) if isinstance(services, dict) else None
    if not isinstance(target, dict):
        raise ValueError("post-load runtime provenance is missing target service")
    if target.get("container_id") != after["container_id"] or target.get("image_id") != after["image_id"]:
        raise ValueError("post-load target provenance does not match recovery evidence")
    if target.get("revision") != expected_revision or target.get("source_dirty") is not False:
        raise ValueError("post-load target source provenance is invalid")

    return {
        "baseline_sha256": baseline_sha256,
        "attempted": attempted,
        "accepted": accepted,
        "persisted": persisted,
        "received": received,
        "expected_receipts": expected_receipts,
        "delivery_rate": 1.0,
        "peak_kafka_lag": peak_lag,
        "settled_kafka_lag": settled_lag,
    }


def build_report(evidence, baseline, baseline_sha256):
    if not isinstance(evidence, dict) or set(evidence) != EVIDENCE_FIELDS:
        raise ValueError("recovery evidence fields do not match v1")
    if evidence["schema_version"] != EVIDENCE_SCHEMA_VERSION:
        raise ValueError("unsupported recovery evidence schema_version")
    run_id = evidence["run_id"]
    target_service = evidence["target_service"]
    expected_revision = evidence["expected_revision"]
    if not isinstance(run_id, str) or not re.fullmatch(r"[A-Za-z0-9._-]+", run_id):
        raise ValueError("run_id is invalid")
    if target_service not in {"dipole-node1", "dipole-node2", "dipole-node3"}:
        raise ValueError("target_service is unsupported")
    if not isinstance(expected_revision, str) or not REVISION_PATTERN.fullmatch(expected_revision):
        raise ValueError("expected_revision must be a full lowercase Git revision")
    if evidence["fault"] != {"action": "stop_start"}:
        raise ValueError("fault must be the stop_start action")

    before = _snapshot(evidence["before"], "before", expected_revision)
    after = _snapshot(evidence["after"], "after", expected_revision)
    if before["image_id"] != after["image_id"]:
        raise ValueError("recovery changed the target image")
    if before["pid"] == after["pid"]:
        raise ValueError("recovery did not replace the target process")

    timeline = evidence["timeline"]
    if not isinstance(timeline, dict) or set(timeline) != TIMELINE_FIELDS:
        raise ValueError("timeline fields do not match recovery evidence v1")
    parsed = {name: _timestamp(value, f"timeline.{name}") for name, value in timeline.items()}
    ordered = [
        parsed["fault_started_at"],
        parsed["unavailable_observed_at"],
        parsed["start_requested_at"],
        parsed["ready_observed_at"],
    ]
    if ordered != sorted(ordered) or len(set(ordered)) != len(ordered):
        raise ValueError("recovery timeline must be strictly ordered")
    unavailable_to_ready_ms = int(
        (parsed["ready_observed_at"] - parsed["unavailable_observed_at"]).total_seconds() * 1000
    )
    restart_to_ready_ms = int(
        (parsed["ready_observed_at"] - parsed["start_requested_at"]).total_seconds() * 1000
    )

    return {
        "schema_version": REPORT_SCHEMA_VERSION,
        "run_id": run_id,
        "target_service": target_service,
        "expected_revision": expected_revision,
        "fault": {"action": "stop_start"},
        "recovery": {
            "fault_to_unavailable_ms": int(
                (parsed["unavailable_observed_at"] - parsed["fault_started_at"]).total_seconds() * 1000
            ),
            "unavailable_to_ready_ms": unavailable_to_ready_ms,
            "restart_to_ready_ms": restart_to_ready_ms,
        },
        "timeline": timeline,
        "before": before,
        "after": after,
        "post_load": _post_load(
            baseline, target_service, expected_revision, after, baseline_sha256
        ),
        "passed": True,
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description="Validate C1 stop/start recovery evidence.")
    parser.add_argument("--evidence", type=Path, required=True)
    parser.add_argument("--baseline", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)

    evidence = json.loads(args.evidence.read_text(encoding="utf-8"))
    baseline_bytes = args.baseline.read_bytes()
    baseline = json.loads(baseline_bytes)
    report = build_report(evidence, baseline, hashlib.sha256(baseline_bytes).hexdigest())
    args.output.write_text(json.dumps(report, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
