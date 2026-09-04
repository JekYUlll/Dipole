#!/usr/bin/env python3
"""Keep OAuth callback materials explicit and the public route disabled by default."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parent.parent
COMPOSE = (ROOT / "deploy/compose/docker-compose.microservices.yml").read_text(encoding="utf-8")
CONFIG = (ROOT / "configs/config.dist.yaml").read_text(encoding="utf-8")


class AgentOAuthCallbackComposeTest(unittest.TestCase):
    def test_default_compose_keeps_callback_route_closed(self) -> None:
        self.assertIn('DIPOLE_GATEWAY_AGENT_OAUTH_CALLBACK_ENABLED: ${DIPOLE_GATEWAY_AGENT_OAUTH_CALLBACK_ENABLED:-false}', COMPOSE)
        self.assertIn('DIPOLE_GATEWAY_AGENT_OAUTH_CALLBACK_TARGET: ${DIPOLE_GATEWAY_AGENT_OAUTH_CALLBACK_TARGET:-http://agent:8091}', COMPOSE)
        for key in (
            'SECRET', 'REDIRECT_URI', 'RUNTIME_KEY_ID', 'RUNTIME_PUBLIC_KEY_FILE',
            'CORRELATION_SECRET', 'BROWSER_SESSION_COOKIE', 'CORRELATION_COOKIE',
        ):
            self.assertIn(f'DIPOLE_GATEWAY_AGENT_OAUTH_CALLBACK_{key}: ${{DIPOLE_GATEWAY_AGENT_OAUTH_CALLBACK_{key}:-}}', COMPOSE)

    def test_sample_config_declares_no_callback_material(self) -> None:
        self.assertIn('agent_oauth_callback_enabled: false', CONFIG)
        for key in (
            'agent_oauth_callback_secret', 'agent_oauth_callback_redirect_uri',
            'agent_oauth_callback_runtime_key_id', 'agent_oauth_callback_runtime_public_key_file',
            'agent_oauth_callback_correlation_secret', 'agent_oauth_callback_browser_session_cookie',
            'agent_oauth_callback_correlation_cookie',
        ):
            self.assertIn(f'{key}: ""', CONFIG)


if __name__ == '__main__':
    unittest.main()
