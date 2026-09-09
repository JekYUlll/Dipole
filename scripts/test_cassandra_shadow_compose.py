#!/usr/bin/env python3
"""Static guard for the removable Cassandra shadow projection profile."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class CassandraShadowComposeTest(unittest.TestCase):
    def setUp(self) -> None:
        self.overlay = (ROOT / "deploy/microservices/cassandra-shadow.yml").read_text(encoding="utf-8")

    def test_profile_runs_an_independent_projector_after_schema_init(self) -> None:
        self.assertEqual(self.overlay.count('profiles: ["cassandra-shadow"]'), 3)
        self.assertIn("cassandra-projector:", self.overlay)
        self.assertIn('entrypoint: ["/app/service"]', self.overlay)
        self.assertIn("${DIPOLE_CASSANDRA_PROJECTOR_IMAGE:-dipole-cassandra-projector:latest}", self.overlay)
        self.assertIn("cassandra-init:\n        condition: service_completed_successfully", self.overlay)
        self.assertIn("kafka:\n        condition: service_healthy", self.overlay)
        self.assertIn("cassandra_shadow_data:/var/lib/cassandra", self.overlay)

    def test_shadow_profile_does_not_override_business_readers_or_enable_hydration(self) -> None:
        for service in ("core:", "message:", "sync:", "gateway:"):
            self.assertNotIn(f"\n  {service}", self.overlay)
        self.assertNotIn("DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION", self.overlay)
        self.assertNotIn("DIPOLE_SYNC_CASSANDRA_SHADOW_HYDRATION", self.overlay)
        self.assertNotIn("DIPOLE_MESSAGE_CASSANDRA", self.overlay)

    def test_projector_is_bound_to_the_private_compose_network_only(self) -> None:
        self.assertNotIn("ports:", self.overlay)
        self.assertIn("DIPOLE_CASSANDRA_HOSTS: cassandra:9042", self.overlay)
        self.assertIn("DIPOLE_KAFKA_BROKERS: kafka:9092", self.overlay)
        self.assertIn("DIPOLE_METRICS_ENABLED: \"true\"", self.overlay)

    def test_microservice_image_builder_can_publish_the_projector_binary(self) -> None:
        builder = (ROOT / "scripts/docker-build-microservice-images.sh").read_text(encoding="utf-8")
        self.assertIn('"cassandra-projector:dipole-cassandra-projector"', builder)
        self.assertIn("cassandra-projector) image_variable=DIPOLE_CASSANDRA_PROJECTOR_IMAGE", builder)


if __name__ == "__main__":
    unittest.main()
