#!/usr/bin/env python3
"""Exercise the fail-closed promotion maintenance window runner."""

from pathlib import Path
import json
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts/run-agent-promotion-window.sh"


class AgentPromotionWindowScriptTest(unittest.TestCase):
    def write_config(self, directory: Path, **changes: object) -> Path:
        certificates = directory / "certificates"
        certificates.mkdir(exist_ok=True)
        env_file = directory / "compose.env"
        env_file.write_text("COMPOSE_PROJECT_NAME=dipole-test\n", encoding="utf-8")
        base = directory / "base.yml"
        agent = directory / "agent.yml"
        overlay = directory / "promotion.yml"
        for path in (base, agent, overlay):
            path.write_text("services: {}\n", encoding="utf-8")
        config = {
            "compose_project": "dipole-test",
            "env_file": str(env_file),
            "compose_files": [str(base), str(agent)],
            "promotion_overlay": str(overlay),
            "certificate_dir": str(certificates),
            "gateway_service": "gateway",
            "tenant_id": "dipole",
        }
        config.update(changes)
        path = directory / "window.json"
        path.write_text(json.dumps(config), encoding="utf-8")
        return path

    def test_dry_run_validates_and_never_calls_docker(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            config = self.write_config(Path(temp_dir))
            result = subprocess.run(["bash", str(SCRIPT), "open", "--config", str(config)], cwd=ROOT, text=True, capture_output=True, check=False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Dry run only", result.stdout)
        self.assertIn("certificate_dir=", result.stdout)

    def test_rejects_secret_extra_and_relative_certificate_path(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            directory = Path(temp_dir)
            config = self.write_config(directory, api_key="must-not-be-here")
            extra = subprocess.run(["bash", str(SCRIPT), "open", "--config", str(config)], cwd=ROOT, text=True, capture_output=True, check=False)
            self.assertEqual(extra.returncode, 2)
            self.assertIn("exactly", extra.stderr)

            config = self.write_config(directory, certificate_dir="relative/certs")
            relative = subprocess.run(["bash", str(SCRIPT), "open", "--config", str(config)], cwd=ROOT, text=True, capture_output=True, check=False)
            self.assertEqual(relative.returncode, 2)
            self.assertIn("must be absolute", relative.stderr)

    def test_declares_overlay_only_for_open_and_restores_base_for_close(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn('overlay_compose=("${compose[@]}" -f "$promotion_overlay")', source)
        self.assertIn('"${overlay_compose[@]}" up -d --no-deps --force-recreate "$gateway_service"', source)
        self.assertIn('"${compose[@]}" up -d --no-deps --force-recreate "$gateway_service"', source)
        self.assertIn('assert_gateway_state true', source)
        self.assertIn('assert_gateway_state false', source)
        self.assertIn('DIPOLE_INTERNAL_CERT_DIR="$certificate_dir"', source)


if __name__ == "__main__":
    unittest.main()
