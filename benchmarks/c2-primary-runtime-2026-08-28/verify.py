#!/usr/bin/env python3
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
RAW = ROOT / "raw"
REVISION = "703725b0185d32a95b83e9fc4c03c5d2402e826a"


def json_lines(name: str) -> list[dict]:
    return [json.loads(line) for line in (RAW / name).read_text().splitlines() if line.strip()]


def group_position(name: str, partition: int) -> tuple[int, int, int]:
    for line in (RAW / name).read_text().splitlines():
        fields = line.split()
        if len(fields) >= 6 and fields[1] == "dipole.message.direct.created" and fields[2] == str(partition):
            return tuple(int(value) for value in fields[3:6])
    raise ValueError(f"partition {partition} missing from {name}")


images = json.loads((RAW / "runtime-images.json").read_text())
evidence = json_lines("primary-evidence.ndjson")
saturation = json_lines("saturation-acks.ndjson")
frames = json_lines("ws-frames.ndjson")
crash_worker = json.loads((RAW / "crash-worker.json").read_text())

commit = [item for item in evidence if item.get("source_event_id") == "E-C2-PRIMARY-COMMIT-2"]
crash = [item for item in evidence if item.get("source_event_id") == "E-C2-PRIMARY-CRASH-1"]
before = group_position("group-before-crash.txt", 5)
after = group_position("group-after-recovery.txt", 5)


def stable_and_legacy(event_id: str) -> bool:
    selected = [frame for frame in frames if frame.get("event_id") == event_id]
    return len(selected) == 2 and sum(bool(frame.get("delivery_id")) for frame in selected) == 1


checks = {
    "clean_same_revision_images": len(images) == 2
    and all(image.get("revision") == REVISION and image.get("source_dirty") == "false" for image in images),
    "terminal_ack_committed": len(commit) == 1
    and commit[0].get("primary_offset_decision") == "commit"
    and commit[0].get("transport_enqueued") == 1,
    "real_queue_saturation": any(
        ack.get("status") == "DELIVERY_ACK_STATUS_PARTIAL"
        and ack.get("pressure") == {"depth": 16, "capacity": 16, "retry_after_ms": 25}
        and ack.get("results", [{}])[0].get("status") == "DELIVERY_RESULT_STATUS_BACKPRESSURED"
        for ack in saturation
    ),
    "crash_retained_offset": before == (1, 2, 1)
    and any(item.get("primary_offset_decision") == "retain" and item.get("error_code") == "node_transport" for item in crash),
    "crash_exit_was_sigkill": crash_worker.get("state") == {"status": "exited", "exit_code": 137},
    "replay_committed_same_coordinate": after == (2, 2, 0)
    and any(
        item.get("partition") == 5
        and item.get("offset") == 1
        and item.get("primary_offset_decision") == "commit"
        and item.get("transport_enqueued") == 1
        for item in crash
    ),
    "stable_delivery_frames_observed": all(
        stable_and_legacy(event_id) for event_id in ("E-C2-PRIMARY-COMMIT-2", "E-C2-PRIMARY-CRASH-1")
    ),
    "shared_nodes_remained_running": len((RAW / "shared-nodes-after.txt").read_text().splitlines()) == 3,
}

parallel_duplicate_observed = checks["stable_delivery_frames_observed"]
report = {
    "schema_version": "c2-primary-runtime-evidence-v1",
    "revision": REVISION,
    "checks": checks,
    "runtime_decision": "eligible" if all(checks.values()) else "invalid",
    "cutover_decision": "blocked" if parallel_duplicate_observed else "review",
    "cutover_blocker": "AD-041: Go and C++ delivery authorities emit parallel client frames"
    if parallel_duplicate_observed
    else "",
    "boundaries": [
        "primary runtime remains absent from tracked Compose defaults",
        "crash occurred after deferred evidence and before offset commit",
        "terminal evidence-to-commit crash window remains probabilistic and was not claimed",
    ],
}
(ROOT / "report.json").write_text(json.dumps(report, indent=2, sort_keys=True) + "\n")
print(json.dumps(report, indent=2, sort_keys=True))
raise SystemExit(0 if all(checks.values()) and parallel_duplicate_observed else 1)
