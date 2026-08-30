#!/usr/bin/env python3
import argparse
import hashlib
import json
import math
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timedelta, timezone
from pathlib import Path


SESSION_SCHEMA = "dipole.web-sync.observation-session.v1"
EVIDENCE_SCHEMA = "dipole.web-sync.observation-evidence.v1"
MINIMUM_WINDOW = timedelta(hours=24)
MAX_FUTURE_SKEW = timedelta(minutes=5)
MINIMUM_MATCHES = 100
ALERT_QUERY = 'count(ALERTS{alertname=~"DipoleSyncProjector(Lag|Retry|DeadLetter)|DipoleWebSync(ShadowDivergence|ShadowOverflow|StorageFull|ClientErrors)",alertstate="firing"})'
START_QUERIES = {
    "comparison_series": 'count(dipole_web_sync_comparison_total{scope="incoming_direct",outcome="match"})',
    "sync_projector_lag": 'sum(kafka_consumergroup_lag{consumergroup="dipole-sync-consumer"})',
    "window_rule_loaded": "dipole:web_sync_shadow:window_complete",
    "firing_alerts": ALERT_QUERY,
}
FINAL_QUERIES = {
    "matches_24h": "dipole:web_sync_shadow:matches_24h",
    "terminal_differences_24h": "dipole:web_sync_shadow:terminal_differences_24h",
    "overflows_24h": "dipole:web_sync_shadow:overflows_24h",
    "window_complete": "dipole:web_sync_shadow:window_complete",
    "promotion_ready": "dipole:web_sync_shadow:promotion_ready",
    "firing_alerts": ALERT_QUERY,
}


class PrometheusClient:
    def __init__(self, base_url, timeout_seconds=10):
        self.base_url = _prometheus_url(base_url)
        self.timeout_seconds = timeout_seconds

    def query(self, expression, captured_at):
        query = urllib.parse.urlencode({"query": expression, "time": _iso(captured_at)})
        request = urllib.request.Request(f"{self.base_url}/api/v1/query?{query}", headers={"Accept": "application/json"})
        try:
            with urllib.request.urlopen(request, timeout=self.timeout_seconds) as response:
                payload = response.read()
        except (urllib.error.URLError, TimeoutError) as error:
            raise RuntimeError(f"query Prometheus: {error}") from error
        try:
            document = json.loads(payload)
        except json.JSONDecodeError as error:
            raise RuntimeError("Prometheus returned invalid JSON") from error
        if document.get("status") != "success":
            raise RuntimeError(f"Prometheus query failed: {document.get('error', 'unknown error')}")
        return document


def build_session(candidate_version, git_commit, bundle_path, prometheus_url, started_at, client, now=None):
    candidate_version = candidate_version.strip()
    git_commit = git_commit.strip().lower()
    if not candidate_version:
        raise ValueError("candidate version is required")
    if not re.fullmatch(r"[a-f0-9]{40}", git_commit):
        raise ValueError("git commit must be a full 40-character SHA-1")
    prometheus_url = _prometheus_url(prometheus_url)
    bundle_path = Path(bundle_path)
    if not bundle_path.is_file():
        raise ValueError("candidate Web bundle is required")
    started_at = _utc(started_at)
    _reject_future_timestamp("session start", started_at, now)
    snapshot = _query_snapshot(client, START_QUERIES, started_at)
    values = snapshot["values"]
    issues = []
    if values["comparison_series"] is None or values["comparison_series"] < 1:
        issues.append("incoming_direct match series is unavailable")
    if values["sync_projector_lag"] is None or values["sync_projector_lag"] != 0:
        issues.append("Sync Projector lag must be zero")
    if values["window_rule_loaded"] not in (0, 1):
        issues.append("Web Sync observation recording rules are unavailable")
    if (values["firing_alerts"] or 0) != 0:
        issues.append("Sync observation alerts must be clear")
    if issues:
        raise ValueError("; ".join(issues))
    candidate = {
        "version": candidate_version,
        "git_commit": git_commit,
        "bundle_path": bundle_path.name,
        "bundle_sha256": _file_sha256(bundle_path),
    }
    identity = {
        "candidate": candidate,
        "prometheus_url": prometheus_url,
        "started_at": _iso(started_at),
        "initial_snapshot": snapshot,
    }
    return {
        "schema_version": SESSION_SCHEMA,
        "session_id": "web-sync:" + _sha256_json(identity),
        "candidate": candidate,
        "prometheus_url": prometheus_url,
        "started_at": _iso(started_at),
        "minimum_end_at": _iso(started_at + MINIMUM_WINDOW),
        "initial_snapshot": snapshot,
    }


def build_final_evidence(session, ended_at, client, now=None, archive=None):
    _validate_session(session)
    started_at = _parse_time(session["started_at"])
    ended_at = _utc(ended_at)
    _reject_future_timestamp("observation end", ended_at, now)
    if ended_at - started_at < MINIMUM_WINDOW:
        raise ValueError("Web Sync observation requires a complete 24-hour window")
    snapshot = _query_snapshot(client, FINAL_QUERIES, ended_at)
    values = snapshot["values"]
    issues = []
    if values["matches_24h"] is None or values["matches_24h"] < MINIMUM_MATCHES:
        issues.append(f"matches must be at least {MINIMUM_MATCHES}")
    if values["terminal_differences_24h"] is None or values["terminal_differences_24h"] != 0:
        issues.append("terminal differences must be zero")
    if values["overflows_24h"] is None or values["overflows_24h"] != 0:
        issues.append("comparator overflows must be zero")
    if values["window_complete"] != 1:
        issues.append("Prometheus 24-hour window is incomplete")
    if values["promotion_ready"] != 1:
        issues.append("promotion recording rule is not ready")
    if (values["firing_alerts"] or 0) != 0:
        issues.append("Sync observation alerts are firing")
    archive = _normalize_archive(archive, now=now)
    if archive is None:
        issues.append("evidence archive receipt is required")
    snapshot_hash = _sha256_json(snapshot)
    evidence = {
        "schema_version": EVIDENCE_SCHEMA,
        "session_id": session["session_id"],
        "session_sha256": _sha256_json(session),
        "candidate": session["candidate"],
        "prometheus_url": session["prometheus_url"],
        "started_at": session["started_at"],
        "ended_at": _iso(ended_at),
        "duration_seconds": int((ended_at - started_at).total_seconds()),
        "decision": "eligible" if not issues else "blocked",
        "issues": issues,
        "final_snapshot": snapshot,
        "snapshot_sha256": snapshot_hash,
        "archive": archive,
    }
    evidence["evidence_sha256"] = _sha256_json(evidence)
    return evidence


def build_status(session, captured_at, client, now=None):
    _validate_session(session)
    captured_at = _utc(captured_at)
    started_at = _parse_time(session["started_at"])
    if captured_at < started_at:
        raise ValueError("Web Sync observation status cannot precede session start")
    _reject_future_timestamp("status capture", captured_at, now)
    snapshot = _query_snapshot(client, FINAL_QUERIES, captured_at)
    return {
        "session_id": session["session_id"],
        "candidate": session["candidate"],
        "captured_at": _iso(captured_at),
        "elapsed_seconds": max(0, int((captured_at - started_at).total_seconds())),
        "minimum_window_complete": captured_at - started_at >= MINIMUM_WINDOW,
        "snapshot": snapshot,
    }


def verify_candidate(session, git_commit, bundle_path):
    _validate_session(session)
    if git_commit.strip().lower() != session["candidate"]["git_commit"]:
        raise ValueError("candidate Git commit changed during observation")
    bundle_path = Path(bundle_path)
    if not bundle_path.is_file() or _file_sha256(bundle_path) != session["candidate"]["bundle_sha256"]:
        raise ValueError("candidate Web bundle changed during observation")


def write_immutable_json(path, document):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("x", encoding="utf-8") as output:
        json.dump(document, output, ensure_ascii=True, sort_keys=True, indent=2)
        output.write("\n")


def _query_snapshot(client, queries, captured_at):
    raw, values = {}, {}
    for name, expression in queries.items():
        response = client.query(expression, captured_at)
        raw[name] = {"expression": expression, "response": response}
        values[name] = _scalar(response)
    return {"captured_at": _iso(captured_at), "values": values, "raw": raw}


def _scalar(response):
    try:
        result = response["data"]["result"]
    except (KeyError, TypeError) as error:
        raise ValueError("Prometheus response has an invalid vector shape") from error
    if len(result) == 0:
        return None
    if len(result) != 1:
        raise ValueError("Prometheus scalar query returned multiple series")
    try:
        value = float(result[0]["value"][1])
    except (KeyError, IndexError, TypeError, ValueError) as error:
        raise ValueError("Prometheus scalar value is invalid") from error
    if not math.isfinite(value):
        raise ValueError("Prometheus scalar value must be finite")
    return int(value) if value.is_integer() else value


def _validate_session(session):
    if session.get("schema_version") != SESSION_SCHEMA:
        raise ValueError("unsupported Web Sync observation session")
    identity = {"candidate": session.get("candidate"), "prometheus_url": session.get("prometheus_url"), "started_at": session.get("started_at"), "initial_snapshot": session.get("initial_snapshot")}
    if session.get("session_id") != "web-sync:" + _sha256_json(identity):
        raise ValueError("Web Sync observation session identity mismatch")
    expected_end = _iso(_parse_time(session["started_at"]) + MINIMUM_WINDOW)
    if session.get("minimum_end_at") != expected_end:
        raise ValueError("Web Sync observation minimum end time mismatch")


def _file_sha256(path):
    digest = hashlib.sha256()
    with Path(path).open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def _sha256_json(document):
    payload = json.dumps(document, ensure_ascii=True, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def _utc(value):
    if value.tzinfo is None:
        raise ValueError("timestamp must include a timezone")
    return value.astimezone(timezone.utc)


def _reject_future_timestamp(label, value, now=None):
    reference = datetime.now(timezone.utc) if now is None else _utc(now)
    if value > reference + MAX_FUTURE_SKEW:
        raise ValueError(f"{label} cannot be more than {int(MAX_FUTURE_SKEW.total_seconds() / 60)} minutes in the future")


def _iso(value):
    return _utc(value).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def _parse_time(value):
    return datetime.fromisoformat(value.replace("Z", "+00:00")).astimezone(timezone.utc)


def _prometheus_url(value):
    parsed = urllib.parse.urlparse(value.strip())
    if parsed.scheme not in ("http", "https") or not parsed.netloc:
        raise ValueError("Prometheus URL must use http or https")
    if parsed.username is not None or parsed.password is not None or parsed.query or parsed.fragment:
        raise ValueError("Prometheus evidence URL cannot contain credentials, query, or fragment")
    return value.rstrip("/")


def _normalize_archive(archive, now=None):
    if archive is None:
        return None
    if not isinstance(archive, dict):
        raise ValueError("evidence archive receipt must be an object")
    required = ("uri", "object_version", "etag", "retention_until")
    if any(not isinstance(archive.get(key), str) or not archive[key].strip() for key in required):
        raise ValueError("evidence archive receipt is incomplete")
    uri = archive["uri"].strip()
    parsed = urllib.parse.urlparse(uri)
    if parsed.scheme not in ("s3", "minio") or not parsed.netloc or parsed.query or parsed.fragment or parsed.username or parsed.password:
        raise ValueError("evidence archive URI must be an opaque s3 or minio URI")
    etag = archive["etag"].strip().strip('"')
    if not re.fullmatch(r"[a-fA-F0-9]{32,64}", etag):
        raise ValueError("evidence archive ETag must be a hexadecimal value")
    try:
        retention_until = _parse_time(archive["retention_until"].strip())
    except (TypeError, ValueError) as error:
        raise ValueError("evidence archive retention_until must include a timezone") from error
    current = datetime.now(timezone.utc) if now is None else _utc(now)
    if retention_until <= current:
        raise ValueError("evidence archive retention_until must be in the future")
    return {
        "uri": uri,
        "object_version": archive["object_version"].strip(),
        "etag": etag.lower(),
        "retention_until": _iso(retention_until),
    }


def _load_json(path):
    return json.loads(Path(path).read_text(encoding="utf-8"))


def _at(value):
    return datetime.now(timezone.utc) if value is None else _parse_time(value)


def main(argv=None):
    parser = argparse.ArgumentParser(description="Manage immutable Web Sync Prometheus observation evidence")
    subparsers = parser.add_subparsers(dest="command", required=True)
    start = subparsers.add_parser("start")
    start.add_argument("--candidate-version", required=True)
    start.add_argument("--git-commit", required=True)
    start.add_argument("--bundle", type=Path, required=True)
    start.add_argument("--prometheus-url", required=True)
    start.add_argument("--output", type=Path, required=True)
    start.add_argument("--at")
    status = subparsers.add_parser("status")
    status.add_argument("--session", type=Path, required=True)
    status.add_argument("--at")
    finalize = subparsers.add_parser("finalize")
    finalize.add_argument("--session", type=Path, required=True)
    finalize.add_argument("--git-commit", required=True)
    finalize.add_argument("--bundle", type=Path, required=True)
    finalize.add_argument("--output", type=Path, required=True)
    finalize.add_argument("--archive-uri", required=True)
    finalize.add_argument("--archive-object-version", required=True)
    finalize.add_argument("--archive-etag", required=True)
    finalize.add_argument("--archive-retain-until", required=True)
    finalize.add_argument("--at")
    args = parser.parse_args(argv)
    try:
        if args.command == "start":
            client = PrometheusClient(args.prometheus_url)
            result = build_session(args.candidate_version, args.git_commit, args.bundle, args.prometheus_url, _at(args.at), client)
            write_immutable_json(args.output, result)
        else:
            session = _load_json(args.session)
            client = PrometheusClient(session["prometheus_url"])
            if args.command == "status":
                result = build_status(session, _at(args.at), client)
            else:
                verify_candidate(session, args.git_commit, args.bundle)
                result = build_final_evidence(
                    session,
                    _at(args.at),
                    client,
                    archive={
                        "uri": args.archive_uri,
                        "object_version": args.archive_object_version,
                        "etag": args.archive_etag,
                        "retention_until": args.archive_retain_until,
                    },
                )
                write_immutable_json(args.output, result)
        print(json.dumps(result, ensure_ascii=True, sort_keys=True, indent=2))
        return 2 if result.get("decision") == "blocked" else 0
    except (FileExistsError, OSError, RuntimeError, ValueError, json.JSONDecodeError) as error:
        print(f"web-sync-observation: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
