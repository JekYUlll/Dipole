#!/usr/bin/env python3
"""Static contract for the default-off Workflow Repair execute overlay."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class WorkflowRepairExecuteComposeTest(unittest.TestCase):
    def test_overlay_only_flips_the_execute_flag(self) -> None:
        overlay = (ROOT / "deploy/microservices/agent-workflow-repair-execute.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_INTERNAL_RPC_AGENT_WORKFLOW_REPAIR_EXECUTE_ENABLED: "true"', overlay)
        self.assertNotIn("DIPOLE_AGENT_CONTROL_ENABLED", overlay)
        self.assertNotIn("DIPOLE_AGENT_TEMPORAL_ENABLED", overlay)
        self.assertNotIn("DIPOLE_GATEWAY_AGENT", overlay)

    def test_default_compose_pins_execute_off(self) -> None:
        base = (ROOT / "deploy/compose/docker-compose.microservices.yml").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_INTERNAL_RPC_AGENT_WORKFLOW_REPAIR_EXECUTE_ENABLED: "false"', base)

    def test_compose_gate_asserts_default_off(self) -> None:
        checker = (ROOT / "scripts/check-compose.sh").read_text(encoding="utf-8")
        self.assertIn('DIPOLE_INTERNAL_RPC_AGENT_WORKFLOW_REPAIR_EXECUTE_ENABLED == "false"', checker)


if __name__ == "__main__":
    unittest.main()
