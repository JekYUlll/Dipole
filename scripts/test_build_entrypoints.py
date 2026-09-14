import subprocess
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class BuildEntrypointsTest(unittest.TestCase):
    def make(self, *args):
        return subprocess.check_output(["make", "-n", *args], cwd=ROOT, text=True)

    def test_default_build_excludes_optional_tools(self):
        output = self.make("build")
        self.assertEqual(output.count(" go build "), 7)
        self.assertNotIn("cassandra", output)
        self.assertNotIn("timeline-repair", output)

    def test_single_image_rebuilds_binary_before_packaging(self):
        output = self.make("image-core", "DIPOLE_CORE_IMAGE=example/core:test")
        self.assertLess(output.index("go build"), output.index("docker build"))
        self.assertIn('"example/core:test"', output)
        self.assertIn("DIPOLE_BINARY=dipole-server", output)
        self.assertEqual(output.count(" go build "), 1)

    def test_optional_tool_is_explicit(self):
        output = self.make("tool-cassandra-backfill")
        self.assertIn("./cmd/tools/cassandra-backfill", output)
        self.assertEqual(output.count(" go build "), 1)

    def test_system_layer_precedes_variable_metadata(self):
        for name in ("Dockerfile", "deploy/images/go-service.Dockerfile"):
            source = (ROOT / name).read_text()
            self.assertLess(source.index("RUN apk"), source.index("ARG DIPOLE_"))


if __name__ == "__main__":
    unittest.main()
