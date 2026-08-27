#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

try:
    from scripts.bench.runtime_provenance import build_report as build_runtime_provenance
except ModuleNotFoundError:
    from runtime_provenance import build_report as build_runtime_provenance


SCHEMA_VERSION = "dipole.performance.baseline.v4"
OPERATIONS_SCHEMA_VERSIONS = {
    "dipole.performance.operations.v1",
    "dipole.performance.operations.v2",
    "dipole.performance.operations.v3",
    "dipole.performance.operations.v4",
}
PROJECTION_NAMES = {"direct_message", "group_message", "group_init"}


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


def _nonnegative_int(value, field):
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        raise ValueError(f"{field} must be a non-negative integer")
    return value


def _unavailable_conversation_state():
    return {
        "available": False,
        "rows_touched": None,
        "messages_observed": None,
        "write_operations": None,
        "writes_per_observed_message": None,
        "projection_writes": None,
        "counter_source": None,
        "timing_available": False,
        "duration_source": None,
        "timing": None,
    }


def _unavailable_process_resources():
    return {
        "available": False,
        "sample_count": None,
        "duration_seconds": None,
        "services": None,
        "counter_source": None,
    }


def _unavailable_runtime_provenance():
    return {
        "available": False,
        "expected_revision": None,
        "source_aligned": None,
        "services": None,
    }


def _nonnegative_number(value, field, nullable=False):
    if value is None and nullable:
        return None
    if isinstance(value, bool) or not isinstance(value, (int, float)) or value < 0:
        suffix = " or null" if nullable else ""
        raise ValueError(f"{field} must be a non-negative number{suffix}")
    return value


def _conversation_timing(values, projection_writes):
    if not isinstance(values, dict) or set(values) != PROJECTION_NAMES:
        raise ValueError("timing fields do not match operations v3")
    result = {}
    expected_fields = {
        "success_count",
        "error_count",
        "success_sum_seconds",
        "average_success_ms",
        "p95_success_upper_bound_ms",
    }
    for projection in sorted(PROJECTION_NAMES):
        raw = values[projection]
        if not isinstance(raw, dict) or set(raw) != expected_fields:
            raise ValueError(f"timing.{projection} fields do not match operations v3")
        success_count = _nonnegative_int(raw["success_count"], f"timing.{projection}.success_count")
        error_count = _nonnegative_int(raw["error_count"], f"timing.{projection}.error_count")
        success_sum_seconds = _nonnegative_number(
            raw["success_sum_seconds"], f"timing.{projection}.success_sum_seconds"
        )
        average_success_ms = _nonnegative_number(
            raw["average_success_ms"], f"timing.{projection}.average_success_ms", nullable=True
        )
        p95_success_upper_bound_ms = _nonnegative_number(
            raw["p95_success_upper_bound_ms"],
            f"timing.{projection}.p95_success_upper_bound_ms",
            nullable=True,
        )
        if success_count != projection_writes[projection]:
            raise ValueError(f"timing.{projection}.success_count must equal projection writes")
        if success_count == 0 and (average_success_ms is not None or p95_success_upper_bound_ms is not None):
            raise ValueError(f"timing.{projection} empty observations must have null latency values")
        if success_count > 0 and average_success_ms is None:
            raise ValueError(f"timing.{projection}.average_success_ms is required")
        result[projection] = {
            "success_count": success_count,
            "error_count": error_count,
            "success_sum_seconds": _rounded(success_sum_seconds, 9),
            "average_success_ms": _rounded(average_success_ms),
            "p95_success_upper_bound_ms": _rounded(p95_success_upper_bound_ms),
        }
    return result


def _conversation_state_section(storage, source_schema_version):
    values = storage.get("conversation_state")
    if not isinstance(values, dict):
        if source_schema_version != "dipole.performance.operations.v1":
            raise ValueError("operations v2/v3 requires storage.conversation_state")
        return _unavailable_conversation_state()

    expected_fields = {
        "rows_touched",
        "messages_observed",
        "write_operations",
        "projection_writes",
        "counter_source",
    }
    if source_schema_version in {
        "dipole.performance.operations.v3",
        "dipole.performance.operations.v4",
    }:
        expected_fields |= {"duration_source", "timing"}
    if set(values) != expected_fields:
        raise ValueError("storage.conversation_state fields do not match operations schema")
    rows_touched = _nonnegative_int(values["rows_touched"], "rows_touched")
    messages_observed = _nonnegative_int(values["messages_observed"], "messages_observed")
    write_operations = _nonnegative_int(values["write_operations"], "write_operations")
    raw_projection_writes = values["projection_writes"]
    if not isinstance(raw_projection_writes, dict) or set(raw_projection_writes) != PROJECTION_NAMES:
        raise ValueError("projection_writes fields do not match operations schema")
    projection_writes = {
        name: _nonnegative_int(raw_projection_writes[name], f"projection_writes.{name}")
        for name in sorted(PROJECTION_NAMES)
    }
    if write_operations != projection_writes["direct_message"] + projection_writes["group_message"]:
        raise ValueError("write_operations must equal direct_message plus group_message writes")
    if values["counter_source"] != "dipole_conversation_projection_writes_total":
        raise ValueError("unsupported conversation counter_source")
    result = {
        "available": True,
        "rows_touched": rows_touched,
        "messages_observed": messages_observed,
        "write_operations": write_operations,
        "writes_per_observed_message": _rounded(
            write_operations / messages_observed if messages_observed > 0 else None
        ),
        "projection_writes": projection_writes,
        "counter_source": values["counter_source"],
        "timing_available": False,
        "duration_source": None,
        "timing": None,
    }
    if source_schema_version in {
        "dipole.performance.operations.v3",
        "dipole.performance.operations.v4",
    }:
        if values["duration_source"] != "dipole_conversation_projection_write_duration_seconds":
            raise ValueError("unsupported conversation duration_source")
        result.update({
            "timing_available": True,
            "duration_source": values["duration_source"],
            "timing": _conversation_timing(values["timing"], projection_writes),
        })
    return result


def _process_resources_section(operations, source_schema_version):
    values = operations.get("process_resources")
    if source_schema_version != "dipole.performance.operations.v4":
        return _unavailable_process_resources()
    if not isinstance(values, dict):
        raise ValueError("operations v4 requires process_resources")
    if set(values) != {"schema_version", "sample_count", "duration_seconds", "services"}:
        raise ValueError("process_resources fields do not match process resources v1")
    if values["schema_version"] != "dipole.performance.process-resources.v1":
        raise ValueError("unsupported process_resources schema_version")

    sample_count = _nonnegative_int(values["sample_count"], "process_resources.sample_count")
    if sample_count < 2:
        raise ValueError("process_resources.sample_count must be at least two")
    duration_seconds = _nonnegative_number(
        values["duration_seconds"], "process_resources.duration_seconds"
    )
    if duration_seconds == 0:
        raise ValueError("process_resources.duration_seconds must be positive")
    raw_services = values["services"]
    if not isinstance(raw_services, dict) or not raw_services:
        raise ValueError("process_resources.services must not be empty")

    expected_fields = {
        "pid",
        "cpu_core_percent",
        "rss_start_bytes",
        "rss_end_bytes",
        "rss_peak_bytes",
        "thread_peak",
        "voluntary_context_switches",
        "involuntary_context_switches",
    }
    services = {}
    for name in sorted(raw_services):
        raw = raw_services[name]
        if not isinstance(name, str) or not name or not isinstance(raw, dict) or set(raw) != expected_fields:
            raise ValueError(f"process_resources.services.{name} fields do not match process resources v1")
        service = {
            "pid": _nonnegative_int(raw["pid"], f"process_resources.services.{name}.pid"),
            "cpu_core_percent": _rounded(_nonnegative_number(
                raw["cpu_core_percent"], f"process_resources.services.{name}.cpu_core_percent"
            )),
            "rss_start_bytes": _nonnegative_int(
                raw["rss_start_bytes"], f"process_resources.services.{name}.rss_start_bytes"
            ),
            "rss_end_bytes": _nonnegative_int(
                raw["rss_end_bytes"], f"process_resources.services.{name}.rss_end_bytes"
            ),
            "rss_peak_bytes": _nonnegative_int(
                raw["rss_peak_bytes"], f"process_resources.services.{name}.rss_peak_bytes"
            ),
            "thread_peak": _nonnegative_int(
                raw["thread_peak"], f"process_resources.services.{name}.thread_peak"
            ),
            "voluntary_context_switches": _nonnegative_int(
                raw["voluntary_context_switches"],
                f"process_resources.services.{name}.voluntary_context_switches",
            ),
            "involuntary_context_switches": _nonnegative_int(
                raw["involuntary_context_switches"],
                f"process_resources.services.{name}.involuntary_context_switches",
            ),
        }
        if service["pid"] == 0:
            raise ValueError(f"process_resources.services.{name}.pid must be positive")
        if service["rss_peak_bytes"] < max(service["rss_start_bytes"], service["rss_end_bytes"]):
            raise ValueError(f"process_resources.services.{name}.rss_peak_bytes is inconsistent")
        services[name] = service

    return {
        "available": True,
        "sample_count": sample_count,
        "duration_seconds": _rounded(duration_seconds),
        "services": services,
        "counter_source": "/proc/<pid>/stat,status,task/*/status",
    }


def _runtime_provenance_section(operations, source_schema_version):
    values = operations.get("runtime_provenance")
    if source_schema_version != "dipole.performance.operations.v4":
        return _unavailable_runtime_provenance()
    if not isinstance(values, dict):
        raise ValueError("operations v4 requires runtime_provenance")
    if set(values) != {"schema_version", "expected_revision", "source_aligned", "services"}:
        raise ValueError("runtime_provenance fields do not match provenance v1")
    if values["schema_version"] != "dipole.performance.runtime-provenance.v1":
        raise ValueError("unsupported runtime_provenance schema_version")
    if values["source_aligned"] is not True:
        raise ValueError("runtime_provenance must be source aligned")
    raw_services = values["services"]
    if not isinstance(raw_services, dict) or not raw_services:
        raise ValueError("runtime_provenance.services must not be empty")
    normalized = build_runtime_provenance(
        values["expected_revision"],
        [dict(service, name=name) for name, service in raw_services.items()],
    )
    collector_revision = operations.get("environment", {}).get("git_commit")
    if collector_revision != normalized["expected_revision"]:
        raise ValueError("runtime_provenance expected_revision must match environment.git_commit")
    return {
        "available": True,
        "expected_revision": normalized["expected_revision"],
        "source_aligned": normalized["source_aligned"],
        "services": normalized["services"],
    }


def build_report(summary, operations):
    source_schema_version = operations.get("schema_version")
    if source_schema_version not in OPERATIONS_SCHEMA_VERSIONS:
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
        "source_schema_version": source_schema_version,
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
            "conversation_state": _conversation_state_section(
                storage,
                source_schema_version,
            ),
        },
        "kafka": {
            "samples": lag_samples,
            "peak_lag": max(lag_samples) if lag_samples else None,
            "settled_lag": lag_samples[-1] if lag_samples else None,
        },
        "process_resources": _process_resources_section(operations, source_schema_version),
        "runtime_provenance": _runtime_provenance_section(operations, source_schema_version),
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
    conversation_state = report["storage"]["conversation_state"]
    if conversation_state["timing_available"]:
        for projection in sorted(PROJECTION_NAMES):
            error_count = conversation_state["timing"][projection]["error_count"]
            if error_count != 0:
                issues.append(
                    f"Conversation projection {projection} observed {error_count} write errors"
                )

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
    conversation_state = report["storage"]["conversation_state"]
    kafka = report["kafka"]
    delivery = report["delivery"]
    workload = report["workload"]
    process_resources = report["process_resources"]
    runtime_provenance = report["runtime_provenance"]
    timing = conversation_state["timing"]
    timing_rows = "Timing evidence unavailable for this source schema."
    if timing is not None:
        labels = {
            "direct_message": "Direct message",
            "group_message": "Group message",
            "group_init": "Group init",
        }
        timing_rows = "\n".join(
            f"| {labels[projection]} | {timing[projection]['success_count']} | "
            f"{timing[projection]['error_count']} | "
            f"{_format_number(timing[projection]['average_success_ms'], ' ms')} | "
            f"{_format_number(timing[projection]['p95_success_upper_bound_ms'], ' ms')} |"
            for projection in ("direct_message", "group_message", "group_init")
        )
    process_rows = "| Evidence unavailable | N/A | N/A | N/A | N/A | N/A |"
    if process_resources["services"] is not None:
        process_rows = "\n".join(
            f"| {name.replace('-', ' ').title()} | "
            f"{_format_number(values['cpu_core_percent'], '%')} | "
            f"{_format_number(values['rss_peak_bytes'] / (1024 * 1024), ' MiB')} | "
            f"{values['thread_peak']} | {values['voluntary_context_switches']} | "
            f"{values['involuntary_context_switches']} |"
            for name, values in process_resources["services"].items()
        )
    provenance_rows = "| Evidence unavailable | N/A | N/A | N/A |"
    if runtime_provenance["services"] is not None:
        provenance_rows = "\n".join(
            f"| {name.replace('-', ' ').title()} | `{values['image_id'][7:19]}` | "
            f"`{values['revision'][:12]}` | {'dirty' if values['source_dirty'] else 'clean'} |"
            for name, values in runtime_provenance["services"].items()
        )

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
| Receiver connection window | {parameters.get('receiver_conn_ms', 'N/A')} ms |
| Sender connection window | {parameters.get('sender_conn_ms', 'N/A')} ms |
| Hot-group warm-up messages | {parameters.get('hot_group_warmup_messages', 'N/A')} |
| Hot-group thresholds | members={parameters.get('hot_group_member_count_threshold', 'N/A')}, messages={parameters.get('hot_group_message_threshold', 'N/A')} |
| Phone namespace | `{parameters.get('phone_prefix', 'N/A')}` |

### Runtime Provenance

Expected revision: `{runtime_provenance['expected_revision'] or 'N/A'}`

Source aligned: {'yes' if runtime_provenance['source_aligned'] else 'no' if runtime_provenance['source_aligned'] is not None else 'N/A'}

| Service | Image ID | Revision | Source tree |
| --- | --- | --- | --- |
{provenance_rows}

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

## Process Resources

Samples: {process_resources['sample_count'] if process_resources['sample_count'] is not None else 'N/A'}

Duration: {_format_number(process_resources['duration_seconds'], ' s')}

Counter source: `{process_resources['counter_source'] or 'N/A'}`

| Service | CPU core | Peak RSS | Peak threads | Voluntary context switches | Involuntary context switches |
| --- | ---: | ---: | ---: | ---: | ---: |
{process_rows}

## Durable Inbox

| Target | Messages | Inbox rows | Write amplification |
| --- | ---: | ---: | ---: |
| Direct | {direct['messages']} | {direct['inbox_rows']} | {_format_number(direct['inbox_write_amplification'])} |
| Group | {group['messages']} | {group['inbox_rows']} | {_format_number(group['inbox_write_amplification'])} |

## Conversation State

| Metric | Value |
| --- | ---: |
| Evidence available | {'yes' if conversation_state['available'] else 'no'} |
| Conversation rows touched | {conversation_state['rows_touched'] if conversation_state['rows_touched'] is not None else 'N/A'} |
| Conversation messages observed | {conversation_state['messages_observed'] if conversation_state['messages_observed'] is not None else 'N/A'} |
| Conversation write operations | {conversation_state['write_operations'] if conversation_state['write_operations'] is not None else 'N/A'} |
| Conversation writes / observed message | {_format_number(conversation_state['writes_per_observed_message'])} |
| Direct-message projection writes | {conversation_state['projection_writes']['direct_message'] if conversation_state['projection_writes'] is not None else 'N/A'} |
| Group-message projection writes | {conversation_state['projection_writes']['group_message'] if conversation_state['projection_writes'] is not None else 'N/A'} |
| Group-init projection writes | {conversation_state['projection_writes']['group_init'] if conversation_state['projection_writes'] is not None else 'N/A'} |
| Counter source | `{conversation_state['counter_source'] or 'N/A'}` |

### Projection Repository Timing

| Projection | Successful calls | Errors | Average | P95 bucket upper bound |
| --- | ---: | ---: | ---: | ---: |
{timing_rows}

Duration source: `{conversation_state['duration_source'] or 'N/A'}`

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
