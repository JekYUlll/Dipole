#!/usr/bin/env python3
import argparse
import json
import math
import re
from pathlib import Path


PROJECTIONS = ("direct_message", "group_init", "group_message")
COUNTER_SOURCE = "dipole_conversation_projection_writes_total"
DURATION_SOURCE = "dipole_conversation_projection_write_duration_seconds"
SAMPLE_RE = re.compile(
    r"^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(.*)\})?\s+([^\s]+)(?:\s+.*)?$"
)
LABEL_RE = re.compile(r'([a-zA-Z_][a-zA-Z0-9_]*)="((?:\\.|[^"\\])*)"')


def _labels(raw):
    if not raw:
        return ()
    labels = {}
    for match in LABEL_RE.finditer(raw):
        labels[match.group(1)] = json.loads(f'"{match.group(2)}"')
    if len(labels) != raw.count('="'):
        raise ValueError(f"cannot parse Prometheus labels: {raw}")
    return tuple(sorted(labels.items()))


def _parse_snapshot(text):
    samples = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        match = SAMPLE_RE.match(line)
        if not match:
            continue
        name, raw_labels, raw_value = match.groups()
        if not name.startswith("dipole_conversation_projection_"):
            continue
        key = (name, _labels(raw_labels))
        samples[key] = samples.get(key, 0.0) + float(raw_value)
    return samples


def _key(name, **labels):
    return name, tuple(sorted(labels.items()))


def _required(samples, name, **labels):
    key = _key(name, **labels)
    if key not in samples:
        rendered = ",".join(f'{key}="{value}"' for key, value in labels.items())
        raise ValueError(f"missing Prometheus sample {name}{{{rendered}}}")
    return samples[key]


def _node_deltas(before_texts, after_texts):
    if len(before_texts) != len(after_texts):
        raise ValueError("before and after snapshot counts must match")
    deltas = []
    for index, (before_text, after_text) in enumerate(zip(before_texts, after_texts)):
        before = _parse_snapshot(before_text)
        after = _parse_snapshot(after_text)
        node_delta = {}
        for key, before_value in before.items():
            if key not in after:
                raise ValueError(f"node {index} is missing a metric after the benchmark")
            after_value = after[key]
            if after_value + 1e-12 < before_value:
                raise ValueError(f"node {index} metric reset during benchmark: {key[0]}")
            node_delta[key] = after_value - before_value
        deltas.append(node_delta)
    return deltas


def _sum(deltas, name, **labels):
    total = 0.0
    for node in deltas:
        total += _required(node, name, **labels)
    return total


def _as_count(value, field):
    rounded = round(value)
    if value < 0 or not math.isclose(value, rounded, abs_tol=1e-9):
        raise ValueError(f"{field} must be a non-negative integer delta")
    return int(rounded)


def _success_buckets(deltas, projection):
    bounds = set()
    for node in deltas:
        for name, labels in node:
            label_map = dict(labels)
            if (
                name == f"{DURATION_SOURCE}_bucket"
                and label_map.get("projection") == projection
                and label_map.get("outcome") == "success"
            ):
                bounds.add(label_map["le"])
    result = []
    for raw_bound in bounds:
        value = _sum(
            deltas,
            f"{DURATION_SOURCE}_bucket",
            projection=projection,
            outcome="success",
            le=raw_bound,
        )
        bound = math.inf if raw_bound == "+Inf" else float(raw_bound)
        result.append((bound, _as_count(value, f"{projection} bucket {raw_bound}")))
    return sorted(result)


def _p95_upper_bound_ms(buckets, count):
    if count == 0:
        return None
    rank = math.ceil(count * 0.95)
    for bound, cumulative_count in buckets:
        if cumulative_count >= rank:
            return None if math.isinf(bound) else round(bound * 1000, 6)
    raise ValueError("duration buckets do not contain the successful observation count")


def build_delta(before_texts, after_texts):
    deltas = _node_deltas(before_texts, after_texts)
    projection_writes = {}
    timing = {}
    for projection in PROJECTIONS:
        writes = _as_count(
            _sum(deltas, COUNTER_SOURCE, projection=projection),
            f"projection_writes.{projection}",
        )
        success_count = _as_count(
            _sum(
                deltas,
                f"{DURATION_SOURCE}_count",
                projection=projection,
                outcome="success",
            ),
            f"timing.{projection}.success_count",
        )
        error_count = _as_count(
            _sum(
                deltas,
                f"{DURATION_SOURCE}_count",
                projection=projection,
                outcome="error",
            ),
            f"timing.{projection}.error_count",
        )
        success_sum = _sum(
            deltas,
            f"{DURATION_SOURCE}_sum",
            projection=projection,
            outcome="success",
        )
        if writes != success_count:
            raise ValueError(
                f"{projection} writes {writes} do not match successful duration count {success_count}"
            )
        projection_writes[projection] = writes
        timing[projection] = {
            "success_count": success_count,
            "error_count": error_count,
            "success_sum_seconds": round(success_sum, 9),
            "average_success_ms": (
                None if success_count == 0 else round(success_sum * 1000 / success_count, 6)
            ),
            "p95_success_upper_bound_ms": _p95_upper_bound_ms(
                _success_buckets(deltas, projection), success_count
            ),
        }
    return {
        "write_operations": projection_writes["direct_message"] + projection_writes["group_message"],
        "projection_writes": projection_writes,
        "counter_source": COUNTER_SOURCE,
        "duration_source": DURATION_SOURCE,
        "timing": timing,
    }


def main():
    parser = argparse.ArgumentParser(description="Diff Conversation projection Prometheus snapshots.")
    parser.add_argument("--before", action="append", required=True, type=Path)
    parser.add_argument("--after", action="append", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    result = build_delta(
        [path.read_text(encoding="utf-8") for path in args.before],
        [path.read_text(encoding="utf-8") for path in args.after],
    )
    args.output.write_text(json.dumps(result, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
