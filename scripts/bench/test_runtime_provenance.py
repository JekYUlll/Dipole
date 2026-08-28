import json
import tempfile
import unittest
from pathlib import Path

from scripts.bench.runtime_provenance import build_report, main


REVISION = "a" * 40
IMAGE_ID = "sha256:" + "b" * 64


def service(name, container_suffix):
    return {
        "name": name,
        "container_id": container_suffix * 64,
        "image_id": IMAGE_ID,
        "revision": REVISION,
        "created": "2026-08-28T06:30:00Z",
        "source_dirty": False,
    }


class RuntimeProvenanceTest(unittest.TestCase):
    def test_builds_aligned_report_for_shared_image(self):
        report = build_report(
            REVISION,
            [service("dipole-node2", "2"), service("dipole-node1", "1")],
        )

        self.assertEqual(report["schema_version"], "dipole.performance.runtime-provenance.v1")
        self.assertEqual(report["expected_revision"], REVISION)
        self.assertTrue(report["source_aligned"])
        self.assertEqual(list(report["services"]), ["dipole-node1", "dipole-node2"])
        self.assertEqual(report["services"]["dipole-node1"]["image_id"], IMAGE_ID)

    def test_rejects_unverifiable_or_misaligned_services(self):
        cases = []

        missing_revision = service("dipole-node1", "1")
        missing_revision["revision"] = ""
        cases.append(missing_revision)

        dirty = service("dipole-node1", "1")
        dirty["source_dirty"] = True
        cases.append(dirty)

        mismatched = service("dipole-node1", "1")
        mismatched["revision"] = "c" * 40
        cases.append(mismatched)

        malformed_image = service("dipole-node1", "1")
        malformed_image["image_id"] = "dipole-server:latest"
        cases.append(malformed_image)

        timezone_missing = service("dipole-node1", "1")
        timezone_missing["created"] = "2026-08-28T06:30:00"
        cases.append(timezone_missing)

        for candidate in cases:
            with self.subTest(candidate=candidate):
                with self.assertRaises(ValueError):
                    build_report(REVISION, [candidate])

    def test_rejects_duplicate_container_bindings(self):
        first = service("dipole-node1", "1")
        second = service("dipole-node2", "1")

        with self.assertRaisesRegex(ValueError, "duplicate container_id"):
            build_report(REVISION, [first, second])

    def test_cli_writes_normalized_json(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "provenance.json"
            status = main([
                "--expected-revision",
                REVISION,
                "--service-json",
                json.dumps(service("dipole-node1", "1")),
                "--output",
                str(output),
            ])

            self.assertEqual(status, 0)
            self.assertEqual(json.loads(output.read_text(encoding="utf-8"))["expected_revision"], REVISION)


if __name__ == "__main__":
    unittest.main()
