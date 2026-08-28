#!/usr/bin/env python3
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parent
RAW = ROOT / "raw"


def last_json_line(name: str) -> dict:
    lines = [line for line in (RAW / name).read_text(encoding="utf-8").splitlines() if line.startswith("{")]
    if not lines:
        raise ValueError(f"{name} has no JSON acknowledgement")
    return json.loads(lines[-1])


first = last_json_line("online-first-ack.txt")
replay = last_json_line("online-replay-ack.txt")
stale = last_json_line("stale-ack.txt")
frames = [json.loads(line) for line in (RAW / "ws-frames.ndjson").read_text(encoding="utf-8").splitlines()]
presence = json.loads((RAW / "presence-sanitized.json").read_text(encoding="utf-8"))
runtime = json.loads((RAW / "gateway-runtime.json").read_text(encoding="utf-8"))
metrics = (RAW / "gateway-metrics.prom").read_text(encoding="utf-8")
kafka_groups = (RAW / "kafka-groups.txt").read_text(encoding="utf-8")

primary_frames = [frame for frame in frames if frame.get("delivery_id") == "D-C2-EVIDENCE-ONLINE"]
checks = {
    "online_ack_enqueued": first["status"] == "DELIVERY_ACK_STATUS_ACCEPTED"
    and first["results"] == [{
        "delivery_id": "D-C2-EVIDENCE-ONLINE",
        "status": "DELIVERY_RESULT_STATUS_ENQUEUED",
        "accepted_connections": 1,
    }],
    "replay_ack_stable": replay["batch_id"] == first["batch_id"]
    and replay["status"] == first["status"]
    and replay["results"] == first["results"],
    "client_received_once": len(primary_frames) == 1
    and primary_frames[0].get("request_id") == "R-C2-EVIDENCE-ONLINE"
    and primary_frames[0].get("trace_id") == "T-C2-EVIDENCE-ONLINE",
    "stale_presence_offline": stale["status"] == "DELIVERY_ACK_STATUS_ACCEPTED"
    and stale["results"] == [{
        "delivery_id": "D-C2-EVIDENCE-STALE",
        "status": "DELIVERY_RESULT_STATUS_OFFLINE",
    }]
    and "CE3B3B9FE2BE59BF07ACD" in presence,
    "online_presence_recorded": "CA7548F19EA2ED8F1C1EC" in presence,
    "gateway_ready": runtime.get("health") == "healthy"
    and 'dipole_service_ready{service="dipole-gateway"} 1' in metrics
    and 'dipole_dependency_ready{dependency="kafka-assignment",service="dipole-gateway"} 1' in metrics,
    "gateway_kafka_assigned": "dipole-gateway-consumer" in kafka_groups,
}
report = {
    "schema_version": "c2-primary-delivery-evidence-v1",
    "source_revision": "9589bbc63a380767ac932561fecf438281de9800",
    "gateway_image_id": runtime.get("image_id"),
    "checks": checks,
    "eligible_for_runtime_primary_review": all(checks.values()),
    "remaining_gates": [
        "real partial WebSocket queue saturation through the C++ probe",
        "Kafka consume-to-ACK offset commit and crash replay in an explicit primary runtime",
    ],
}
(ROOT / "report.json").write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
if not report["eligible_for_runtime_primary_review"]:
    raise SystemExit(1)
