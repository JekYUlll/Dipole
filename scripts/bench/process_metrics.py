#!/usr/bin/env python3
import argparse
import json
import os
import time
from pathlib import Path


SAMPLE_SCHEMA_VERSION = "dipole.performance.process-sample.v1"
REPORT_SCHEMA_VERSION = "dipole.performance.process-resources.v1"


def _nonnegative_int(value, field):
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        raise ValueError(f"{field} must be a non-negative integer")
    return value


def _parse_status(path):
    values = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        if ":" not in line:
            continue
        key, raw = line.split(":", 1)
        values[key] = raw.strip()
    return values


def _status_int(values, key, suffix=""):
    raw = values.get(key, "")
    if suffix and raw.endswith(suffix):
        raw = raw[: -len(suffix)].strip()
    try:
        value = int(raw)
    except ValueError as exc:
        raise ValueError(f"missing or invalid {key}") from exc
    return _nonnegative_int(value, key)


def _read_process(pid, proc_root=Path("/proc")):
    process_root = proc_root / str(pid)
    raw_stat = (process_root / "stat").read_text(encoding="utf-8").strip()
    close_paren = raw_stat.rfind(")")
    if close_paren < 0:
        raise ValueError(f"invalid stat for pid {pid}")
    fields = raw_stat[close_paren + 2 :].split()
    if len(fields) < 20:
        raise ValueError(f"incomplete stat for pid {pid}")

    status = _parse_status(process_root / "status")
    voluntary = 0
    involuntary = 0
    task_paths = sorted((process_root / "task").glob("*/status"))
    if not task_paths:
        task_paths = [process_root / "status"]
    for task_path in task_paths:
        task_status = _parse_status(task_path)
        voluntary += _status_int(task_status, "voluntary_ctxt_switches")
        involuntary += _status_int(task_status, "nonvoluntary_ctxt_switches")

    return {
        "pid": pid,
        "start_time_ticks": _nonnegative_int(int(fields[19]), "start_time_ticks"),
        "cpu_ticks": _nonnegative_int(int(fields[11]) + int(fields[12]), "cpu_ticks"),
        "rss_bytes": _status_int(status, "VmRSS", "kB") * 1024,
        "thread_count": _status_int(status, "Threads"),
        "voluntary_context_switches": voluntary,
        "involuntary_context_switches": involuntary,
    }


def capture_sample(services, proc_root=Path("/proc")):
    if not services:
        raise ValueError("at least one service is required")
    return {
        "schema_version": SAMPLE_SCHEMA_VERSION,
        "captured_monotonic_ns": time.monotonic_ns(),
        "clock_ticks_per_second": os.sysconf("SC_CLK_TCK"),
        "services": {
            name: _read_process(pid, proc_root)
            for name, pid in sorted(services.items())
        },
    }


def summarize_samples(samples):
    if len(samples) < 2:
        raise ValueError("at least two process samples are required")

    first = samples[0]
    service_names = set(first.get("services", {}))
    if not service_names:
        raise ValueError("process samples require services")
    clock_ticks = _nonnegative_int(first.get("clock_ticks_per_second"), "clock_ticks_per_second")
    if clock_ticks == 0:
        raise ValueError("clock_ticks_per_second must be positive")

    previous_ns = None
    for index, sample in enumerate(samples):
        if sample.get("schema_version") != SAMPLE_SCHEMA_VERSION:
            raise ValueError(f"sample {index} has unsupported schema_version")
        if set(sample.get("services", {})) != service_names:
            raise ValueError(f"sample {index} service set changed")
        if sample.get("clock_ticks_per_second") != clock_ticks:
            raise ValueError(f"sample {index} clock tick rate changed")
        captured_ns = _nonnegative_int(sample.get("captured_monotonic_ns"), "captured_monotonic_ns")
        if previous_ns is not None and captured_ns <= previous_ns:
            raise ValueError("sample timestamps must increase")
        previous_ns = captured_ns

    duration_seconds = (samples[-1]["captured_monotonic_ns"] - first["captured_monotonic_ns"]) / 1e9
    if duration_seconds <= 0:
        raise ValueError("sample duration must be positive")

    services = {}
    counters = (
        "cpu_ticks",
        "voluntary_context_switches",
        "involuntary_context_switches",
    )
    for name in sorted(service_names):
        observations = [sample["services"][name] for sample in samples]
        identity = (observations[0].get("pid"), observations[0].get("start_time_ticks"))
        for observation in observations:
            if (observation.get("pid"), observation.get("start_time_ticks")) != identity:
                raise ValueError(f"{name} process identity changed")
            for field in (
                "pid",
                "start_time_ticks",
                "cpu_ticks",
                "rss_bytes",
                "thread_count",
                "voluntary_context_switches",
                "involuntary_context_switches",
            ):
                _nonnegative_int(observation.get(field), f"{name}.{field}")
        for field in counters:
            previous = observations[0][field]
            for observation in observations[1:]:
                current = observation[field]
                if current < previous:
                    raise ValueError(f"{name} {field} regressed")
                previous = current

        cpu_seconds = (observations[-1]["cpu_ticks"] - observations[0]["cpu_ticks"]) / clock_ticks
        services[name] = {
            "pid": identity[0],
            "cpu_core_percent": round(cpu_seconds / duration_seconds * 100, 6),
            "rss_start_bytes": observations[0]["rss_bytes"],
            "rss_end_bytes": observations[-1]["rss_bytes"],
            "rss_peak_bytes": max(item["rss_bytes"] for item in observations),
            "thread_peak": max(item["thread_count"] for item in observations),
            "voluntary_context_switches": (
                observations[-1]["voluntary_context_switches"]
                - observations[0]["voluntary_context_switches"]
            ),
            "involuntary_context_switches": (
                observations[-1]["involuntary_context_switches"]
                - observations[0]["involuntary_context_switches"]
            ),
        }

    return {
        "schema_version": REPORT_SCHEMA_VERSION,
        "sample_count": len(samples),
        "duration_seconds": round(duration_seconds, 6),
        "services": services,
    }


def _parse_services(values):
    services = {}
    for value in values:
        name, separator, raw_pid = value.partition("=")
        if not separator or not name or name in services:
            raise ValueError(f"invalid service binding: {value}")
        try:
            pid = int(raw_pid)
        except ValueError as exc:
            raise ValueError(f"invalid service pid: {value}") from exc
        if pid <= 0:
            raise ValueError(f"invalid service pid: {value}")
        services[name] = pid
    return services


def _load_samples(path):
    samples = []
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if not line.strip():
            continue
        try:
            samples.append(json.loads(line))
        except json.JSONDecodeError as exc:
            raise ValueError(f"invalid JSON sample at line {line_number}") from exc
    return samples


def main():
    parser = argparse.ArgumentParser(description="Capture and summarize Linux process resource evidence")
    subparsers = parser.add_subparsers(dest="command", required=True)

    capture = subparsers.add_parser("capture")
    capture.add_argument("--service", action="append", default=[], metavar="NAME=PID")
    capture.add_argument("--output", type=Path, required=True)

    summarize = subparsers.add_parser("summarize")
    summarize.add_argument("--input", type=Path, required=True)
    summarize.add_argument("--output", type=Path, required=True)

    args = parser.parse_args()
    try:
        if args.command == "capture":
            sample = capture_sample(_parse_services(args.service))
            args.output.parent.mkdir(parents=True, exist_ok=True)
            with args.output.open("a", encoding="utf-8") as handle:
                handle.write(json.dumps(sample, sort_keys=True, separators=(",", ":")) + "\n")
        else:
            report = summarize_samples(_load_samples(args.input))
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    except (OSError, ValueError) as exc:
        parser.error(str(exc))


if __name__ == "__main__":
    main()
