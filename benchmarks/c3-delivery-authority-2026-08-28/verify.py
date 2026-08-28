#!/usr/bin/env python3
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent
RAW = ROOT / "raw"
SOURCE_REVISION = "7d079f659e2113197b7f07ae0427af77e6597277"
ARCHIVE_REVISION = "bbea83f13ca8a6f010c787baf22e70f4e92989c0"
TOPIC = "dipole.message.direct.created"


def load_json(name: str) -> dict:
    return json.loads((RAW / name).read_text())


def load_json_lines(name: str) -> list[dict]:
    return [json.loads(line) for line in (RAW / name).read_text().splitlines() if line.strip()]


def metric_enabled(name: str, authority: str) -> bool:
    pattern = rf'^dipole_realtime_delivery_authority\{{authority="{authority}"\}} 1$'
    return bool(re.search(pattern, (RAW / name).read_text(), re.MULTILINE))


def nonempty_topic_positions(name: str) -> list[tuple[int, int, int]]:
    positions = []
    for line in (RAW / name).read_text().splitlines():
        fields = line.split()
        if len(fields) < 6 or fields[1] != TOPIC or fields[3] == "-":
            continue
        positions.append(tuple(int(value) for value in fields[3:6]))
    return positions


def all_at_log_end(name: str) -> bool:
    positions = nonempty_topic_positions(name)
    return bool(positions) and all(current == log_end and lag == 0 for current, log_end, lag in positions)


def running_shared_nodes(name: str) -> set[str]:
    nodes = set()
    for line in (RAW / name).read_text().splitlines():
        fields = line.split()
        if len(fields) >= 2 and fields[1] == "Up":
            nodes.add(fields[0])
    return nodes


go_probe = load_json("go-probe.json")
cpp_probe = load_json("cpp-probe.json")
ready = load_json("cpp-realtime-ready.json")
health = load_json("cpp-container-health.json")
provenance = load_json("runtime-provenance.json")
primary_evidence = load_json_lines("primary.ndjson")
cpp_delivery_ids = cpp_probe.get("delivery_ids", [])
target_event = cpp_delivery_ids[0].split(":", 1)[0] if len(cpp_delivery_ids) == 1 else ""
terminal = [item for item in primary_evidence if item.get("source_event_id") == target_event]
expected_nodes = {"dipole-node1", "dipole-node2", "dipole-node3"}
cleanup = dict(
    line.split("=", 1) for line in (RAW / "cleanup.txt").read_text().splitlines() if "=" in line
)

checks = {
    "go_exactly_one_legacy_frame": go_probe.get("mode") == "go"
    and go_probe.get("matching_frames") == 1
    and go_probe.get("delivery_ids") == [""],
    "cpp_exactly_one_stable_frame": cpp_probe.get("mode") == "cpp"
    and cpp_probe.get("matching_frames") == 1
    and len(cpp_delivery_ids) == 1
    and bool(cpp_delivery_ids[0]),
    "authority_metrics_match_modes": metric_enabled("go-metrics.prom", "go")
    and metric_enabled("cpp-gateway-metrics.prom", "cpp"),
    "go_authority_reached_log_end": all_at_log_end("go-group.txt"),
    "cpp_checkpoint_reached_log_end": all_at_log_end("cpp-go-group.txt"),
    "cpp_primary_reached_log_end": all_at_log_end("cpp-primary-group.txt"),
    "cpp_terminal_delivery_committed": len(terminal) == 1
    and terminal[0].get("primary_offset_decision") == "commit"
    and terminal[0].get("transport_enqueued") == 1
    and terminal[0].get("transport_mode") == "primary",
    "cpp_application_ready": ready == {
        "status": "ok",
        "service": "dipole-realtime-delivery",
        "mode": "primary",
    },
    "compose_health_false_positive_preserved": health.get("Status") == "unhealthy"
    and any("/dev/tcp/127.0.0.1/8092" in item.get("Output", "") for item in health.get("Log", [])),
    "isolated_resources_removed": cleanup == {
        "isolated_containers": "0",
        "isolated_volumes": "0",
        "isolated_networks": "0",
    },
    "shared_nodes_remained_running": running_shared_nodes("shared-nodes-before-cleanup.txt")
    == expected_nodes
    and running_shared_nodes("shared-nodes-after.txt") == expected_nodes,
    "binary_provenance_bound": provenance.get("binary_source_revision") == SOURCE_REVISION
    and provenance.get("archive_revision") == ARCHIVE_REVISION
    and provenance.get("gateway_binary_sha256")
    == "8ebbad94899084fdd3df3b46d703e1978e45d0c51c05c956e5c3929ba98f3a9e"
    and provenance.get("cpp_binary_sha256")
    == "81631371e1f19935ec9b6feb2160ef83a671f467671764fc61eb18efe8b55644",
}

report = {
    "schema_version": "c3-delivery-authority-evidence-v1",
    "archive_revision": ARCHIVE_REVISION,
    "binary_source_revision": SOURCE_REVISION,
    "checks": checks,
    "local_authority_decision": "eligible" if all(checks.values()) else "invalid",
    "shared_cutover_decision": "blocked",
    "shared_cutover_blocker": (
        "AD-041: shared fencing, coordinated checkpoint receipt and automatic rollback remain open"
    ),
    "boundaries": [
        "the drill used isolated per-mode Compose projects",
        "the Docker health failure came from a temporary /bin/sh probe while application readiness succeeded",
        "no shared dynamic cutover or crash/rebalance/Redis-failure drill is claimed",
    ],
}
(ROOT / "report.json").write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
print(json.dumps(report, indent=2, sort_keys=True))
raise SystemExit(0 if all(checks.values()) else 1)
