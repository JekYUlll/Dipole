#!/usr/bin/env python3
"""Static contract for the Search projection-only Compose profile."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class SearchShadowComposeTest(unittest.TestCase):
    def setUp(self) -> None:
        self.overlay = (ROOT / "deploy/microservices/search-shadow.yml").read_text(encoding="utf-8")

    def test_shadow_profile_starts_only_index_dependencies(self) -> None:
        self.assertEqual(self.overlay.count('profiles: ["search-shadow"]'), 2)
        self.assertIn("elasticsearch:", self.overlay)
        self.assertIn("search-indexer:", self.overlay)
        self.assertNotIn("\n  search:\n", self.overlay)

    def test_gateway_search_route_is_forced_closed(self) -> None:
        self.assertIn("gateway:", self.overlay)
        self.assertIn('DIPOLE_SEARCH_ENABLED: "false"', self.overlay)

    def test_base_profile_keeps_the_indexer_and_query_service_separate(self) -> None:
        base = (ROOT / "deploy/compose/docker-compose.microservices.yml").read_text(encoding="utf-8")
        self.assertIn('search-indexer:\n    profiles: ["search"]', base)
        self.assertIn('search:\n    profiles: ["search"]', base)
        self.assertIn('DIPOLE_SEARCH_ENABLED: ${DIPOLE_SEARCH_ENABLED:-false}', base)

    def test_compose_gate_forces_gateway_search_closed(self) -> None:
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn("search_shadow_config", checker)
        self.assertIn("DIPOLE_SEARCH_ENABLED=true", checker)
        self.assertIn("deploy/microservices/search-shadow.yml", checker)
        self.assertIn('.services.gateway.environment.DIPOLE_SEARCH_ENABLED == "false"', checker)
        self.assertIn("and .services.search == null", checker)


if __name__ == "__main__":
    unittest.main()
