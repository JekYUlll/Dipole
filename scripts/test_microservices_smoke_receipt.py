#!/usr/bin/env python3
"""Static contract checks for the optional low-sensitivity Agent smoke receipt."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]
SOURCE = (ROOT / "scripts" / "smoke-microservices.sh").read_text(encoding="utf-8")


class AgentSmokeReceiptContractTest(unittest.TestCase):
    def test_smoke_requires_terminal_task_convergence(self) -> None:
        self.assertIn(
            "agent_tasks WHERE trigger_ref='${agent_message_id}' AND status='completed'",
            SOURCE,
        )
        self.assertNotIn(
            "agent_tasks WHERE trigger_ref='${agent_message_id}' AND status='running'",
            SOURCE,
        )

    def test_receipt_is_explicit_and_low_sensitivity(self) -> None:
        self.assertIn('AGENT_SMOKE_RECEIPT="${DIPOLE_AGENT_SMOKE_RECEIPT:-}"', SOURCE)
        self.assertIn("'schemaVersion', 'dipole.agent.smoke-receipt.v1'", SOURCE)
        for field in ("'eventId'", "'taskId'", "'runId'", "'traceId'", "'runStatus'"):
            self.assertIn(field, SOURCE)

    def test_receipt_does_not_export_sensitive_payload_fields(self) -> None:
        receipt_block = SOURCE.split('if [[ -n "${AGENT_SMOKE_RECEIPT}" ]]', 1)[1].split('if [[ -n "${AGENT_CORE_RESTART_EVIDENCE}" ]]', 1)[0]
        self.assertNotIn('payload.content', receipt_block)
        self.assertNotIn('model_output', receipt_block)
        self.assertNotIn('DIPOLE_AGENT_MODEL_API_KEY', receipt_block)


if __name__ == "__main__":
    unittest.main()
