import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class BusinessTopologyContractTest(unittest.TestCase):
    def test_microservices_compose_is_explicitly_single_node(self):
        compose = (ROOT / "deploy/compose/docker-compose.microservices.yml").read_text(encoding="utf-8")

        self.assertIn("KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093", compose)
        self.assertIn("DIPOLE_AGENT_KAFKA_BROKERS: kafka:9092", compose)
        self.assertIn("DIPOLE_REALTIME_KAFKA_BROKERS: kafka:9092", compose)
        self.assertIn("DIPOLE_REALTIME_REDIS_ENDPOINT: redis:6379", compose)
        self.assertIn("DIPOLE_REDIS_HOST: redis", compose)

    def test_cluster_files_are_infrastructure_only_until_business_overlay_exists(self):
        kafka = (ROOT / "deploy/compose/docker-compose.cluster.yml").read_text(encoding="utf-8")
        redis = (ROOT / "deploy/compose/docker-compose.redis-cluster.yml").read_text(encoding="utf-8")
        docs = (ROOT / "docs/architecture/BUSINESS-TOPOLOGY.md").read_text(encoding="utf-8")

        self.assertIn("KAFKA_NODE_ID: 1", kafka)
        self.assertIn("KAFKA_NODE_ID: 2", kafka)
        self.assertIn("KAFKA_NODE_ID: 3", kafka)
        self.assertIn("sentinel-1:", redis)
        self.assertIn("docker-compose.business-cluster.yml", docs)
        self.assertIn("业务消息链路的自动故障切换、恢复收敛和可执行回滚 receipt 仍属于后续", docs)

    def test_compose_checker_keeps_business_failover_claims_fail_closed(self):
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn("BUSINESS-TOPOLOGY.md", checker)
        self.assertIn("BUSINESS-TOPOLOGY.md", checker)
        self.assertIn("docker-compose.business-cluster.yml", checker)
        self.assertIn(".source? | (type == \"string\" and endswith", checker)

    def test_business_topology_lifecycle_is_isolated_and_non_destructive(self):
        script = (ROOT / "scripts/bench/business_cluster_topology.sh").read_text(encoding="utf-8")
        self.assertIn("--project-name", script)
        self.assertIn("BUSINESS_CLUSTER_ALLOW_ACTIVE", script)
        self.assertIn("nvidia-smi", script)
        self.assertIn("config)", script)
        self.assertIn("up)", script)
        self.assertIn("status)", script)
        self.assertIn("down)", script)
        self.assertIn("down --remove-orphans", script)
        self.assertNotIn("down --volumes", script)

    def test_business_cluster_can_override_gateway_host_port(self):
        compose = (ROOT / "deploy/compose/docker-compose.microservices.yml").read_text(encoding="utf-8")
        script = (ROOT / "scripts/bench/business_cluster_topology.sh").read_text(encoding="utf-8")
        self.assertIn("${DIPOLE_GATEWAY_PORT:-8080}:8080", compose)
        self.assertIn("BUSINESS_CLUSTER_GATEWAY_PORT", script)
        self.assertIn("18080", script)


if __name__ == "__main__":
    unittest.main()
