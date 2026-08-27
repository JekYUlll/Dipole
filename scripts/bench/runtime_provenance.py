#!/usr/bin/env python3
import argparse
from datetime import datetime
import json
from pathlib import Path
import re


SCHEMA_VERSION = "dipole.performance.runtime-provenance.v1"
REVISION_PATTERN = re.compile(r"^[0-9a-f]{40}$")
CONTAINER_ID_PATTERN = re.compile(r"^[0-9a-f]{64}$")
IMAGE_ID_PATTERN = re.compile(r"^sha256:[0-9a-f]{64}$")
SERVICE_FIELDS = {
    "name",
    "container_id",
    "image_id",
    "revision",
    "created",
    "source_dirty",
}


def _timestamp(value, field):
    if not isinstance(value, str) or not value:
        raise ValueError(f"{field} must be an RFC3339 timestamp")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as error:
        raise ValueError(f"{field} must be an RFC3339 timestamp") from error
    if parsed.tzinfo is None:
        raise ValueError(f"{field} must include an RFC3339 timezone")
    return value


def build_report(expected_revision, raw_services):
    if not isinstance(expected_revision, str) or not REVISION_PATTERN.fullmatch(expected_revision):
        raise ValueError("expected_revision must be a full lowercase Git revision")
    if not isinstance(raw_services, list) or not raw_services:
        raise ValueError("at least one runtime service is required")

    services = {}
    container_ids = set()
    for raw in raw_services:
        if not isinstance(raw, dict) or set(raw) != SERVICE_FIELDS:
            raise ValueError("runtime service fields do not match provenance v1")
        name = raw["name"]
        if not isinstance(name, str) or not name or name in services:
            raise ValueError("runtime service names must be unique non-empty strings")
        container_id = raw["container_id"]
        if not isinstance(container_id, str) or not CONTAINER_ID_PATTERN.fullmatch(container_id):
            raise ValueError(f"services.{name}.container_id must be a full Docker container ID")
        if container_id in container_ids:
            raise ValueError(f"duplicate container_id: {container_id}")
        container_ids.add(container_id)

        image_id = raw["image_id"]
        if not isinstance(image_id, str) or not IMAGE_ID_PATTERN.fullmatch(image_id):
            raise ValueError(f"services.{name}.image_id must be a sha256 image ID")
        revision = raw["revision"]
        if not isinstance(revision, str) or not REVISION_PATTERN.fullmatch(revision):
            raise ValueError(f"services.{name}.revision must be a full lowercase Git revision")
        if revision != expected_revision:
            raise ValueError(
                f"services.{name}.revision {revision} does not match expected {expected_revision}"
            )
        source_dirty = raw["source_dirty"]
        if not isinstance(source_dirty, bool):
            raise ValueError(f"services.{name}.source_dirty must be a boolean")
        if source_dirty:
            raise ValueError(f"services.{name} was built from a dirty source tree")

        services[name] = {
            "container_id": container_id,
            "image_id": image_id,
            "revision": revision,
            "created": _timestamp(raw["created"], f"services.{name}.created"),
            "source_dirty": source_dirty,
        }

    return {
        "schema_version": SCHEMA_VERSION,
        "expected_revision": expected_revision,
        "source_aligned": True,
        "services": {name: services[name] for name in sorted(services)},
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description="Validate runtime Docker image provenance.")
    parser.add_argument("--expected-revision", required=True)
    parser.add_argument("--service-json", action="append", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)

    raw_services = [json.loads(value) for value in args.service_json]
    report = build_report(args.expected_revision, raw_services)
    args.output.write_text(json.dumps(report, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
