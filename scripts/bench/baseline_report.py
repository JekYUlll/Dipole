#!/usr/bin/env python3
import argparse
import json
from pathlib import Path


SCHEMA_VERSION = "dipole.performance.baseline.v1"
OPERATIONS_SCHEMA_VERSION = "dipole.performance.operations.v1"


def _metric_values(summary, name):
    metric = summary.get("metrics", {}).get(name, {})
    return metric.get("values", metric)


def _number(values, *names):
    for name in names:
        value = values.get(name)
        if isinstance(value, (int, float)):
            return value
    return None


def _rounded(value, digits=6):
    return None if value is None else round(value, digits)


def _storage_section(storage, name):
    values = storage.get(name, {})
    messages = int(values.get("messages", 0))
    inbox_rows = int(values.get("inbox_rows", 0))
    amplification = None if messages == 0 else inbox_rows / messages
    return {
        "messages": messages,
        "inbox_rows": inbox_rows,
        "inbox_write_amplification": _rounded(amplification),
    }


def build_report(summary, operations):
    if operations.get("schema_version") != OPERATIONS_SCHEMA_VERSION:
        raise ValueError("unsupported operations schema_version")

    attempted_values = _metric_values(summary, "msg_attempted_total")
    accepted_values = _metric_values(summary, "msg_accepted_total")
    if not accepted_values:
        accepted_values = _metric_values(summary, "msg_sent_total")
    rejected_values = _metric_values(summary, "msg_rejected_total")
    received_values = _metric_values(summary, "msg_received_total")
    expected_values = _metric_values(summary, "msg_expected_receipts_total")
    delivery_values = _metric_values(summary, "msg_delivery_rate")
    latency_values = _metric_values(summary, "msg_e2e_latency_ms")
    http_failure_values = _metric_values(summary, "http_req_failed")
    lag_samples = [int(value) for value in operations.get("kafka_lag_samples", [])]
    storage = operations.get("storage", {})

    received = int(_number(received_values, "count") or 0)
    expected = int(_number(expected_values, "count") or 0)
    measured_delivery_rate = received / expected if expected > 0 else _number(delivery_values, "rate", "value")
    attempted = int(_number(attempted_values, "count") or 0)
    accepted = int(_number(accepted_values, "count") or 0)
    rejected = int(_number(rejected_values, "count") or 0)
    persisted = sum(int(storage.get(name, {}).get("messages", 0)) for name in ("direct", "group"))

    return {
        "schema_version": SCHEMA_VERSION,
        "run_id": operations.get("run_id", ""),
        "scenario": operations.get("scenario", ""),
        "captured_at": operations.get("captured_at"),
        "environment": operations.get("environment", {}),
        "parameters": operations.get("parameters", {}),
        "workload": {
            "attempted": attempted,
            "accepted": accepted,
            "rejected": rejected,
            "persisted": persisted,
            "received": received,
            "expected_receipts": expected,
            "acceptance_rate": _rounded(accepted / attempted) if attempted > 0 else None,
            "persistence_rate": _rounded(persisted / accepted) if accepted > 0 else None,
            "throughput_per_second": _rounded(_number(accepted_values, "rate")),
        },
        "delivery": {
            "rate": _rounded(measured_delivery_rate),
            "http_failure_rate": _rounded(_number(http_failure_values, "rate", "value")),
        },
        "latency_ms": {
            "average": _rounded(_number(latency_values, "avg")),
            "p50": _rounded(_number(latency_values, "p(50)", "med")),
            "p95": _rounded(_number(latency_values, "p(95)")),
            "p99": _rounded(_number(latency_values, "p(99)")),
            "maximum": _rounded(_number(latency_values, "max")),
        },
        "storage": {
            "direct": _storage_section(storage, "direct"),
            "group": _storage_section(storage, "group"),
        },
        "kafka": {
            "samples": lag_samples,
            "peak_lag": max(lag_samples) if lag_samples else None,
            "settled_lag": lag_samples[-1] if lag_samples else None,
        },
    }


def evaluate_report(report, minimum_delivery_rate=0.9):
    workload = report["workload"]
    delivery_rate = report["delivery"]["rate"]
    settled_lag = report["kafka"]["settled_lag"]
    issues = []

    if workload["accepted"] != workload["attempted"]:
        issues.append(
            f"accepted {workload['accepted']} of {workload['attempted']} attempted messages"
        )
    if workload["rejected"] != 0:
        issues.append(f"observed {workload['rejected']} rejected messages")
    if workload["persisted"] != workload["accepted"]:
        issues.append(
            f"persisted {workload['persisted']} of {workload['accepted']} accepted messages"
        )
    if delivery_rate is None:
        issues.append("delivery rate is unavailable")
    elif delivery_rate < minimum_delivery_rate:
        issues.append(
            f"delivery rate {delivery_rate:.6f} is below {minimum_delivery_rate:.6f}"
        )
    if settled_lag is None:
        issues.append("Kafka settled lag is unavailable")
    elif settled_lag != 0:
        issues.append(f"Kafka settled lag is {settled_lag}")

    return issues


def _format_number(value, suffix="", digits=2):
    if value is None:
        return "N/A"
    return f"{value:.{digits}f}{suffix}"


def render_markdown(report):
    environment = report["environment"]
    parameters = report["parameters"]
    latency = report["latency_ms"]
    direct = report["storage"]["direct"]
    group = report["storage"]["group"]
    kafka = report["kafka"]
    delivery = report["delivery"]
    workload = report["workload"]

    return f"""# Dipole Performance Baseline

Run ID: `{report['run_id']}`

Scenario: `{report['scenario']}`

Captured at: `{report.get('captured_at') or 'N/A'}`

## Environment

| Field | Value |
| --- | --- |
| Git commit | `{environment.get('git_commit', 'N/A')}` |
| CPU | {environment.get('cpu', 'N/A')} |
| Topology | `{environment.get('topology', 'N/A')}` |
| Benchmark script | `{parameters.get('bench_script', 'N/A')}` |
| Users | {parameters.get('user_count', 'N/A')} |
| Group size | {parameters.get('group_size', 'N/A')} |
| Senders | {parameters.get('sender_count', 'N/A')} |
| Messages per sender | {parameters.get('messages_per_sender', 'N/A')} |
| Hot-group warm-up messages | {parameters.get('hot_group_warmup_messages', 'N/A')} |
| Hot-group thresholds | members={parameters.get('hot_group_member_count_threshold', 'N/A')}, messages={parameters.get('hot_group_message_threshold', 'N/A')} |
| Phone namespace | `{parameters.get('phone_prefix', 'N/A')}` |

## Workload

| Metric | Value |
| --- | ---: |
| Attempted | {workload['attempted']} |
| Accepted | {workload['accepted']} |
| Rejected | {workload['rejected']} |
| Persisted | {workload['persisted']} |
| Received | {workload['received']} |
| Expected receipts | {workload['expected_receipts']} |
| Accepted throughput | {_format_number(workload['throughput_per_second'], ' msg/s')} |
| Acceptance rate | {_format_number(None if workload['acceptance_rate'] is None else workload['acceptance_rate'] * 100, '%')} |
| Persistence rate | {_format_number(None if workload['persistence_rate'] is None else workload['persistence_rate'] * 100, '%')} |
| Delivery rate | {_format_number(None if delivery['rate'] is None else delivery['rate'] * 100, '%')} |
| HTTP failure rate | {_format_number(None if delivery['http_failure_rate'] is None else delivery['http_failure_rate'] * 100, '%')} |

## End-to-End Latency

| Metric | Value |
| --- | ---: |
| Average | {_format_number(latency['average'], ' ms')} |
| P50 | {_format_number(latency['p50'], ' ms')} |
| P95 | {_format_number(latency['p95'], ' ms')} |
| P99 | {_format_number(latency['p99'], ' ms')} |
| Maximum | {_format_number(latency['maximum'], ' ms')} |

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | {direct['messages']} | {direct['inbox_rows']} | {_format_number(direct['inbox_write_amplification'])} |
| Group | {group['messages']} | {group['inbox_rows']} | {_format_number(group['inbox_write_amplification'])} |

## Kafka Lag

| Metric | Value |
| --- | ---: |
| Peak sampled lag | {kafka['peak_lag'] if kafka['peak_lag'] is not None else 'N/A'} |
| Settled sampled lag | {kafka['settled_lag'] if kafka['settled_lag'] is not None else 'N/A'} |

该报告只描述本次环境、拓扑与负载参数，跨机器或跨版本比较时应保持这些条件一致。
"""


def main():
    parser = argparse.ArgumentParser(description="Build a normalized Dipole performance baseline report.")
    parser.add_argument("summary", type=Path)
    parser.add_argument("operations", type=Path)
    parser.add_argument("output_json", type=Path)
    parser.add_argument("output_markdown", type=Path)
    parser.add_argument("--enforce", action="store_true")
    parser.add_argument("--minimum-delivery-rate", type=float, default=0.9)
    args = parser.parse_args()

    summary = json.loads(args.summary.read_text(encoding="utf-8"))
    operations = json.loads(args.operations.read_text(encoding="utf-8"))
    report = build_report(summary, operations)
    args.output_json.write_text(json.dumps(report, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")
    args.output_markdown.write_text(render_markdown(report), encoding="utf-8")
    if args.enforce:
        issues = evaluate_report(report, args.minimum_delivery_rate)
        if issues:
            for issue in issues:
                print(f"baseline gate failed: {issue}")
            raise SystemExit(1)


if __name__ == "__main__":
    main()
