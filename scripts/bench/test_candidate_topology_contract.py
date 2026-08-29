import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


class CandidateTopologyContractTest(unittest.TestCase):
    def test_dist_compose_exposes_isolation_controls_with_legacy_defaults(self):
        compose = (ROOT / "deploy/compose/docker-compose.dist.yml").read_text(encoding="utf-8")

        self.assertEqual(compose.count("image: ${DIPOLE_IMAGE:-dipole-server:latest}"), 3)
        for suffix in ("mysql", "redis", "kafka", "kafdrop", "minio", "minio-init", "node1", "node2", "node3", "nginx"):
            self.assertIn(f"container_name: ${{DIPOLE_CONTAINER_PREFIX:-dipole}}-{suffix}", compose)
        for variable in (
            "DIPOLE_MYSQL_PORT",
            "DIPOLE_REDIS_PORT",
            "DIPOLE_KAFKA_EXTERNAL_PORT",
            "DIPOLE_NODE1_PORT",
            "DIPOLE_NODE2_PORT",
            "DIPOLE_NODE3_PORT",
            "DIPOLE_HTTP_PORT",
            "DIPOLE_HTTPS_PORT",
            "DIPOLE_NETWORK_SUBNET",
        ):
            self.assertIn(variable, compose)

    def test_candidate_script_pins_image_and_has_non_destructive_rollback(self):
        script = (ROOT / "scripts/bench/candidate_topology.sh").read_text(encoding="utf-8")

        self.assertIn('case "${1:-}" in', script)
        self.assertIn("up)", script)
        self.assertIn("status)", script)
        self.assertIn("down)", script)
        self.assertIn("docker image inspect", script)
        self.assertIn("status --porcelain --untracked-files=no", script)
        self.assertIn("org.opencontainers.image.revision", script)
        self.assertIn("io.dipole.source.dirty", script)
        self.assertIn('DIPOLE_IMAGE="${image_id}"', script)
        self.assertIn("DIPOLE_AI_RUNTIME_MODE=off", script)
        self.assertIn("/app/dipole-migrate", script)
        self.assertIn("--wait-timeout", script)
        self.assertLess(script.index("/app/dipole-migrate"), script.index("dipole-node1 dipole-node2"))
        self.assertIn("docker compose", script)
        self.assertNotIn("down --volumes", script)

    def test_isolated_message_flow_checks_timeline_reads(self):
        script = (ROOT / "scripts/smoke-microservice-isolated-images.sh").read_text(encoding="utf-8")

        self.assertIn("before_seq=0&limit=20", script)
        self.assertIn("after_seq=0&limit=20", script)
        self.assertIn("message_seq", script)
        self.assertIn("user_sync_inbox", script)

    def test_inbox_projector_overlay_enables_projector_validation_in_message_service(self):
        overlay = (ROOT / "deploy/microservices/inbox-projector.yml").read_text(encoding="utf-8")

        self.assertIn("DIPOLE_MESSAGE_INBOX_WRITE_MODE: projector", overlay)
        self.assertIn('DIPOLE_SYNC_PROJECTOR_ENABLED: "true"', overlay)

    def test_canonical_compose_gate_renders_candidate_overrides(self):
        script = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")

        self.assertIn("candidate-compose-validation-only", script)
        self.assertIn("DIPOLE_NETWORK_SUBNET=10.201.0.0/24", script)
        self.assertIn(".services[\"dipole-node1\"].ports[0].published", script)


if __name__ == "__main__":
    unittest.main()
