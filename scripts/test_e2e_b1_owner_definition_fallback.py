#!/usr/bin/env python3
"""Static guard for the live owner-Definition fallback regression script."""

from pathlib import Path
import stat
import unittest


class OwnerDefinitionFallbackScriptTest(unittest.TestCase):
    def test_script_exercises_ungranted_definition_and_lowrisk_fallback(self) -> None:
        script = Path(__file__).with_name("e2e-b1-owner-definition-fallback.sh")
        content = script.read_text(encoding="utf-8")
        self.assertTrue(script.stat().st_mode & stat.S_IXUSR)
        self.assertIn("/api/v1/agent/definitions", content)
        self.assertIn("agent_runtime_promotion_grants", content)
        self.assertIn("lowrisk-assistant:v1", content)
        self.assertIn("message.assistant_reply.send", content)
        self.assertIn("client_message_id", content)


if __name__ == "__main__":
    unittest.main()
