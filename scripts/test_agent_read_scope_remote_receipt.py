#!/usr/bin/env python3
"""Regression contract for the low-sensitivity Agent read-scope receipt."""

import json
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]
RECEIPT = ROOT / "benchmarks/agent-read-scope-confirmation-2026-09-02/receipt.json"


class AgentReadScopeRemoteReceiptTest(unittest.TestCase):
    def test_receipt_records_the_owner_scope_boundary(self) -> None:
        receipt = json.loads(RECEIPT.read_text(encoding="utf-8"))

        self.assertEqual(receipt["schemaVersion"], "dipole.agent.read-scope-remote-receipt.v1")
        self.assertEqual(receipt["result"], "eligible")
        self.assertEqual(receipt["fixture"]["visibleConversationCount"], 2)
        self.assertFalse(receipt["fixture"]["messageWriteEnabled"])
        self.assertEqual(receipt["gateway"], {
            "interactiveTaskStart": 202,
            "forgedInput": 409,
            "forgedInputPreservedWaitingState": True,
            "confirmedInput": 202,
        })
        self.assertEqual(receipt["confirmation"], {
            "initialTaskStatus": "waiting_input",
            "candidateCount": 2,
            "confirmedTaskStatus": "completed",
        })
        self.assertEqual(receipt["effects"], {
            "completedTaskCount": 1,
            "completedRunCount": 1,
            "completedStepCount": 2,
            "conversationListSteps": 1,
            "conversationReadSteps": 1,
            "selectedConversationReads": 1,
            "unconfirmedConversationReads": 0,
            "conversationDigestArtifacts": 1,
        })
        self.assertEqual(receipt["cancellation"], {
            "initialTaskStatus": "waiting_input",
            "ownerCancel": 202,
            "terminalTaskStatus": "cancelled",
            "reason": "user_cancelled",
            "completedTaskCount": 1,
            "completedRunCount": 1,
            "completedListSteps": 1,
            "plannedReadSteps": 1,
            "completedReadSteps": 0,
            "authorizedReadSteps": 0,
        })

    def test_receipt_has_no_fixture_identifiers(self) -> None:
        contents = RECEIPT.read_text(encoding="utf-8")
        for forbidden in ("SCOPE_", "task:", "run:", "conversation_key", "access_token", "telephone"):
            self.assertNotIn(forbidden, contents)


if __name__ == "__main__":
    unittest.main()
