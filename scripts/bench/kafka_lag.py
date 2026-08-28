#!/usr/bin/env python3
import argparse
import sys


def parse_total_lag(output, group_prefix):
    if not isinstance(group_prefix, str) or not group_prefix:
        raise ValueError("group_prefix must be non-empty")
    total = 0
    matched = 0
    for line in output.splitlines():
        fields = line.split()
        if len(fields) < 6 or not fields[0].startswith(group_prefix):
            continue
        matched += 1
        current_offset, log_end_offset, lag = fields[3:6]
        if lag.isdigit():
            total += int(lag)
            continue
        if current_offset == "-" and log_end_offset.isdigit():
            total += int(log_end_offset)
            continue
        raise ValueError(f"cannot parse lag row for group {fields[0]}")
    if matched == 0:
        raise ValueError(f"no consumer group rows match prefix {group_prefix}")
    return total


def main(argv=None):
    parser = argparse.ArgumentParser(description="Aggregate Kafka consumer lag conservatively.")
    parser.add_argument("--group-prefix", required=True)
    args = parser.parse_args(argv)
    print(parse_total_lag(sys.stdin.read(), args.group_prefix))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
