#!/usr/bin/env python3
"""Static safety contract for the isolated Agent OTel smoke."""

import os
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class AgentOtelSmokeTest(unittest.TestCase):
    def test_smoke_is_executable_and_bounds_compose_startup(self) -> None:
        path = ROOT / "scripts/smoke-agent-otel.sh"
        smoke = path.read_text(encoding="utf-8")

        self.assertTrue(os.access(path, os.X_OK))
        self.assertIn("DIPOLE_AGENT_OTEL_SMOKE_STARTUP_TIMEOUT_SECONDS", smoke)
        self.assertIn("Agent OTel smoke startup timeout must be between 30 and 1800 seconds", smoke)
        self.assertIn("command -v timeout", smoke)
        self.assertIn('timeout --preserve-status "${startup_timeout_seconds}s" "${compose[@]}" up -d tempo otel-collector', smoke)

    def test_smoke_uses_inert_model_config_only_for_compose_interpolation(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-otel.sh").read_text(encoding="utf-8")

        self.assertIn("export DIPOLE_AGENT_MODEL_PROVIDER_NAME=compose-smoke", smoke)
        self.assertIn("export DIPOLE_AGENT_MODEL_BASE_URL=http://127.0.0.1:9/v1", smoke)
        self.assertIn("export DIPOLE_AGENT_MODEL_API_KEY=otel-smoke-only", smoke)
        self.assertIn("export DIPOLE_AGENT_MODEL_ROUTES=compose-smoke/deterministic", smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_CONTEXT_PROFILES=', smoke)

    def test_timeout_keeps_the_existing_isolation_and_cleanup(self) -> None:
        smoke = (ROOT / "scripts/smoke-agent-otel.sh").read_text(encoding="utf-8")

        self.assertIn('project_name="dipole-agent-otel-${RANDOM}"', smoke)
        self.assertIn('docker compose -p "$project_name"', smoke)
        self.assertIn('"${compose[@]}" down --volumes --remove-orphans', smoke)
        self.assertIn("trap cleanup EXIT", smoke)


if __name__ == "__main__":
    unittest.main()
