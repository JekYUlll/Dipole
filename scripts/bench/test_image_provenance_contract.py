import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


class ImageProvenanceContractTest(unittest.TestCase):
    def test_dockerfile_publishes_source_labels(self):
        dockerfile = (ROOT / "Dockerfile").read_text(encoding="utf-8")

        self.assertIn("ARG DIPOLE_VCS_REVISION", dockerfile)
        self.assertIn("org.opencontainers.image.revision", dockerfile)
        self.assertIn("org.opencontainers.image.created", dockerfile)
        self.assertIn("io.dipole.source.dirty", dockerfile)

    def test_build_script_passes_frozen_source_metadata(self):
        script = (ROOT / "scripts/docker-build.sh").read_text(encoding="utf-8")

        self.assertIn("--build-arg DIPOLE_VCS_REVISION=", script)
        self.assertIn("--build-arg DIPOLE_BUILD_CREATED=", script)
        self.assertIn("--build-arg DIPOLE_VCS_DIRTY=", script)
        self.assertNotIn('DIPOLE_VCS_REVISION="${DIPOLE_VCS_REVISION:-', script)
        self.assertNotIn('DIPOLE_VCS_DIRTY="${DIPOLE_VCS_DIRTY:-', script)

    def test_benchmark_resolves_running_image_provenance_before_load(self):
        script = (ROOT / "scripts/bench/run_bench.sh").read_text(encoding="utf-8")
        resolve_index = script.index("\nresolve_process_metric_bindings\n")
        load_index = script.index("k6 run")

        self.assertLess(resolve_index, load_index)
        self.assertIn("docker image inspect", script)
        self.assertIn("runtime_provenance.py", script)
        self.assertIn("--expected-revision", script)
        self.assertIn("NODE1_HEALTH_URL", script)
        self.assertIn("NODE2_HEALTH_URL", script)
        self.assertNotIn("for port in 8081 8082", script)


if __name__ == "__main__":
    unittest.main()
