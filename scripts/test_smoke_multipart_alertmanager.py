#!/usr/bin/env python3
"""Static safety contract for the isolated Multipart alert routing smoke."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class MultipartAlertmanagerSmokeTest(unittest.TestCase):
    def test_smoke_is_isolated_and_uses_a_generated_rule(self) -> None:
        smoke = (ROOT / "scripts/smoke-multipart-alertmanager-routing.sh").read_text(encoding="utf-8")
        self.assertIn('PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-multipart-alertmanager-', smoke)
        self.assertIn('multipart-alertmanager-smoke.yml', smoke)
        self.assertIn('--config.file=/smoke/prometheus-services.yml', smoke)
        self.assertIn('DIPOLE_MULTIPART_ALERTMANAGER_SMOKE_DIR}:/smoke:ro', smoke)
        self.assertIn('chmod 755 "${scratch_dir}"', smoke)
        self.assertIn('chmod 644 "${prometheus_config}" "${smoke_rule}" "${compose_override}"', smoke)
        self.assertIn('DipoleMultipartAlertmanagerRoutingSmoke', smoke)
        self.assertIn('expr: vector(1)', smoke)
        self.assertIn('timeout --preserve-status "${timeout_seconds}s" docker compose', smoke)
        self.assertIn('--profile observability up -d --wait prometheus alertmanager', smoke)
        self.assertIn('compose --profile observability down -v --remove-orphans', smoke)

    def test_smoke_reuses_real_rules_and_has_a_bounded_wait(self) -> None:
        smoke = (ROOT / "scripts/smoke-multipart-alertmanager-routing.sh").read_text(encoding="utf-8")
        self.assertIn('/etc/prometheus/multipart-alerts.yml', smoke)
        self.assertIn('DIPOLE_MULTIPART_ALERTMANAGER_SMOKE_TIMEOUT_SECONDS', smoke)
        self.assertIn('must be between 30 and 300 seconds', smoke)
        self.assertIn('DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_BASE_URL:=https://models.invalid/v1', smoke)
        self.assertIn('DIPOLE_AGENT_MODEL_API_KEY:=multipart-alertmanager-smoke-no-network', smoke)
        self.assertIn('sleep 1', smoke)

    def test_smoke_keeps_the_development_discard_receiver(self) -> None:
        smoke = (ROOT / "scripts/smoke-multipart-alertmanager-routing.sh").read_text(encoding="utf-8")
        self.assertIn('alertmanager.yml', smoke)
        self.assertNotIn('webhook_configs', smoke)
        self.assertNotIn('storage.multipart_mode=', smoke)


if __name__ == "__main__":
    unittest.main()
