#!/usr/bin/env python3
"""Safety contract for the isolated External MCP Shadow drill entrypoint."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "drill-agent-external-mcp-shadow.sh"


class AgentExternalMcpShadowDrillTest(unittest.TestCase):
    def test_rejects_node_older_than_22_before_compose(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            node = Path(temp_dir) / "node18"
            node.write_text("#!/usr/bin/env bash\nprintf 'v18.19.1\\n'\n", encoding="utf-8")
            node.chmod(0o700)
            result = subprocess.run(
                [str(SCRIPT)],
                cwd=ROOT,
                env={**os.environ, "DIPOLE_NODE_BIN": str(node)},
                text=True,
                capture_output=True,
                check=False,
            )

        self.assertEqual(result.returncode, 2)
        self.assertIn("requires Node 22+", result.stderr)
        self.assertNotIn("Docker", result.stderr)

    def test_node_gate_precedes_compose_startup(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn('node_version="$("${node_path}" --version', source)
        self.assertIn("requires Node 22+", source)
        self.assertLess(source.index("node_version="), source.index("compose up -d --wait"))

    def test_lockfile_install_skips_nonessential_audit_network(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn('"$npm_bin" ci --ignore-scripts --no-audit --no-fund', source)

    def test_enters_repository_root_before_go_module_commands(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        self.assertIn('cd "$root_dir"', source)
        self.assertLess(source.index('cd "$root_dir"'), source.index('go test "$root_dir/internal/bootstrap"'))


if __name__ == "__main__":
    unittest.main()
