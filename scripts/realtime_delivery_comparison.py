#!/usr/bin/env python3
import argparse
import hashlib
import json
import re
import sys
from pathlib import Path


REPORT_SCHEMA = "dipole.realtime.delivery-comparison-report.v1"
BASELINE_SCHEMA = "dipole.performance.baseline.v4"
EVIDENCE_SCHEMA = "dipole.realtime.shadow-evidence.v3"
SHA1_RE = re.compile(r"[a-f0-9]{40}")


def build_report_from_files(go_baseline_path, cpp_evidence_path, go_revision, cpp_revision):
    baseline_path = Path(go_baseline_path)
    evidence_path = Path(cpp_evidence_path)
    try:
        baseline = json.loads(baseline_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise ValueError(f"read Go baseline: {error}") from error
    records = _load_ndjson(evidence_path)
    return build_report(
        baseline,
        records,
        go_revision,
        cpp_revision,
        input_hashes={
            "go_baseline_sha256": _file_sha256(baseline_path),
            "cpp_evidence_sha256": _file_sha256(evidence_path),
        },
    )


def build_report(baseline, evidence_records, go_revision, cpp_revision, input_hashes=None):
    go_revision = _revision(go_revision, "Go")
    cpp_revision = _revision(cpp_revision, "C++")
    workload = _validate_baseline(baseline, go_revision)
    attempts = _validate_and_group_evidence(evidence_records)
    final_records = [group[-1] for group in attempts]

    issues = []
    accepted = workload["accepted"]
    unique_count = len(final_records)
    if accepted != unique_count:
        issues.append("Go accepted messages must equal unique C++ Kafka records")
    if not all(record["outcome"] == "projected" for record in final_records):
        issues.append("every C++ Kafka record must finish projected")
    if any(record["transport_requested"] == 0 for record in final_records):
        issues.append("every final C++ record must request at least one node batch")
    requested = sum(record["transport_requested"] for record in final_records)
    observed = sum(record["transport_observed"] for record in final_records)
    backpressured = sum(record["transport_backpressured"] for record in final_records)
    rejected = sum(record["transport_rejected"] for record in final_records)
    if requested != observed:
        issues.append("final C++ transport observed count must equal requested count")
    if backpressured != 0 or rejected != 0:
        issues.append("final C++ transport must have zero backpressure and rejection")
    if any(record["presence_malformed"] != 0 or record["presence_stale"] != 0 for record in final_records):
        issues.append("final C++ Presence projection must have zero malformed and stale connections")
    if any(record["offline_item_count"] != 0 for record in final_records):
        issues.append("final C++ comparison workload must have zero offline items")

    for key in ("attempted", "accepted", "persisted", "received", "expected_receipts"):
        if workload[key] != accepted:
            issues.append(f"Go workload {key} must equal accepted messages")
    if workload["rejected"] != 0:
        issues.append("Go workload must have zero rejected messages")
    if baseline["delivery"].get("rate") != 1 or baseline["delivery"].get("http_failure_rate") != 0:
        issues.append("Go delivery and HTTP success rates must be complete")
    if baseline["kafka"].get("settled_lag") != 0:
        issues.append("Go Kafka lag must settle to zero")

    deferred_attempts = sum(
        1 for group in attempts for record in group if record["outcome"] == "deferred"
    )
    duplicate = sum(record["transport_duplicate"] for record in final_records)
    report = {
        "schema_version": REPORT_SCHEMA,
        "decision": "eligible" if not issues else "blocked",
        "issues": issues,
        "run_id": baseline["run_id"],
        "candidates": {"go_revision": go_revision, "cpp_revision": cpp_revision},
        "inputs": input_hashes or {},
        "go_workload": {
            key: workload[key]
            for key in ("attempted", "accepted", "rejected", "persisted", "received", "expected_receipts")
        },
        "comparison": {
            "unique_kafka_records": unique_count,
            "deferred_attempts": deferred_attempts,
            "final_projected": sum(record["outcome"] == "projected" for record in final_records),
            "final_transport_requested": requested,
            "final_transport_observed": observed,
            "final_transport_duplicate": duplicate,
            "final_transport_rejected": rejected,
            "final_transport_backpressured": backpressured,
        },
    }
    report["report_sha256"] = _sha256_json(report)
    return report


def _validate_baseline(baseline, go_revision):
    if not isinstance(baseline, dict) or baseline.get("schema_version") != BASELINE_SCHEMA:
        raise ValueError("unsupported Go baseline schema")
    if not isinstance(baseline.get("run_id"), str) or not baseline["run_id"].strip():
        raise ValueError("Go baseline run_id is required")
    if baseline.get("scenario") not in ("direct", "direct_msg", "concurrent"):
        raise ValueError("Go comparison scenario must be direct or concurrent")
    environment = baseline.get("environment")
    if not isinstance(environment, dict) or environment.get("git_commit") != go_revision:
        raise ValueError("Go revision does not match baseline")
    workload = baseline.get("workload")
    if not isinstance(workload, dict):
        raise ValueError("Go baseline workload is required")
    for key in ("attempted", "accepted", "rejected", "persisted", "received", "expected_receipts"):
        if not isinstance(workload.get(key), int) or isinstance(workload.get(key), bool) or workload[key] < 0:
            raise ValueError(f"Go workload {key} must be a non-negative integer")
    if workload["attempted"] != workload["accepted"] + workload["rejected"]:
        raise ValueError("Go attempted count must equal accepted plus rejected")
    if not isinstance(baseline.get("delivery"), dict) or not isinstance(baseline.get("kafka"), dict):
        raise ValueError("Go delivery and Kafka evidence are required")
    return workload


def _validate_and_group_evidence(records):
    if not isinstance(records, list) or not records:
        raise ValueError("C++ evidence must contain at least one record")
    groups = {}
    event_coordinates = {}
    required_counts = (
        "transport_requested",
        "transport_observed",
        "transport_duplicate",
        "transport_rejected",
        "transport_backpressured",
        "presence_malformed",
        "presence_stale",
        "offline_item_count",
    )
    for index, record in enumerate(records):
        if not isinstance(record, dict) or record.get("schema_version") != EVIDENCE_SCHEMA:
            raise ValueError(f"unsupported C++ evidence schema at line {index + 1}")
        topic, partition, offset = record.get("topic"), record.get("partition"), record.get("offset")
        if not isinstance(topic, str) or not topic or not _non_negative_int(partition) or not _non_negative_int(offset):
            raise ValueError(f"invalid C++ Kafka coordinate at line {index + 1}")
        if record.get("outcome") not in ("projected", "deferred", "rejected"):
            raise ValueError(f"invalid C++ outcome at line {index + 1}")
        for key in required_counts:
            if not _non_negative_int(record.get(key)):
                raise ValueError(f"C++ evidence {key} must be non-negative at line {index + 1}")
        event_id = record.get("source_event_id")
        if not isinstance(event_id, str) or not event_id.strip():
            raise ValueError(f"C++ source event is required at line {index + 1}")
        coordinate = (topic, partition, offset)
        prior_coordinate = event_coordinates.setdefault(event_id, coordinate)
        if prior_coordinate != coordinate:
            raise ValueError("C++ source event maps to multiple Kafka coordinates")
        group = groups.setdefault(coordinate, [])
        if group and (group[0].get("source_event_id") != event_id or group[0].get("batch_id") != record.get("batch_id")):
            raise ValueError("C++ retry identity drifted at one Kafka coordinate")
        group.append(record)
    return list(groups.values())


def _load_ndjson(path):
    try:
        lines = Path(path).read_text(encoding="utf-8").splitlines()
    except OSError as error:
        raise ValueError(f"read C++ evidence: {error}") from error
    records = []
    for line_number, line in enumerate(lines, 1):
        if not line.strip():
            continue
        try:
            records.append(json.loads(line))
        except json.JSONDecodeError as error:
            raise ValueError(f"decode C++ evidence line {line_number}: {error}") from error
    return records


def _revision(value, label):
    value = str(value).strip().lower()
    if not SHA1_RE.fullmatch(value):
        raise ValueError(f"{label} revision must be a full Git SHA-1")
    return value


def _non_negative_int(value):
    return isinstance(value, int) and not isinstance(value, bool) and value >= 0


def _file_sha256(path):
    digest = hashlib.sha256()
    with Path(path).open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def _sha256_json(value):
    encoded = json.dumps(value, ensure_ascii=True, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest()


def main(argv=None):
    parser = argparse.ArgumentParser(description="Build a Go/C++ realtime shadow comparison report")
    parser.add_argument("--go-baseline", required=True)
    parser.add_argument("--cpp-evidence", required=True)
    parser.add_argument("--go-revision", required=True)
    parser.add_argument("--cpp-revision", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args(argv)
    try:
        report = build_report_from_files(
            args.go_baseline, args.cpp_evidence, args.go_revision, args.cpp_revision
        )
        output = Path(args.output)
        output.parent.mkdir(parents=True, exist_ok=True)
        with output.open("x", encoding="utf-8") as destination:
            json.dump(report, destination, ensure_ascii=True, sort_keys=True, indent=2)
            destination.write("\n")
    except (OSError, ValueError) as error:
        print(error, file=sys.stderr)
        return 1
    print(json.dumps({"decision": report["decision"], "report_sha256": report["report_sha256"]}))
    return 0 if report["decision"] == "eligible" else 2


if __name__ == "__main__":
    raise SystemExit(main())
