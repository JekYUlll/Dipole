import hashlib
import json
import stat
import subprocess
import tarfile
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "package-web-sync-bundle.sh"


class WebSyncBundleTest(unittest.TestCase):
    def test_bundle_is_reproducible_and_contains_versioned_manifest(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "dist"
            source.mkdir()
            (source / "index.html").write_text("shadow", encoding="utf-8")
            first = Path(directory) / "first.tar"
            second = Path(directory) / "second.tar"
            for output in (first, second):
                subprocess.run(
                    [
                        str(SCRIPT),
                        "--candidate-version",
                        "web-sync-test",
                        "--mode",
                        "shadow",
                        "--source-dir",
                        str(source),
                        "--output",
                        str(output),
                    ],
                    check=True,
                    cwd=ROOT,
                    capture_output=True,
                    text=True,
                )
            self.assertEqual(hashlib.sha256(first.read_bytes()).digest(), hashlib.sha256(second.read_bytes()).digest())
            self.assertEqual(first.stat().st_mode & 0o777, 0o600)
            with tarfile.open(first) as archive:
                manifest = json.loads(archive.extractfile("./web-sync-bundle.json").read())
                self.assertEqual(manifest["schema_version"], "dipole.web-sync.bundle.v1")
                self.assertEqual(manifest["candidate_version"], "web-sync-test")
                self.assertEqual(manifest["mode"], "shadow")
                self.assertEqual(len(manifest["git_commit"]), 40)

    def test_existing_output_and_dirty_worktree_fail_closed(self):
        source = SCRIPT.read_text(encoding="utf-8")
        for required in ("diff --quiet", "diff --cached --quiet", "refusing to overwrite bundle", "output must be outside the source directory", "chmod 0600", "--sort=name"):
            self.assertIn(required, source)
        self.assertTrue(SCRIPT.stat().st_mode & stat.S_IXUSR)
        subprocess.run(["bash", "-n", str(SCRIPT)], check=True)

    def test_output_inside_source_directory_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "dist"
            source.mkdir()
            (source / "index.html").write_text("shadow", encoding="utf-8")
            result = subprocess.run(
                [str(SCRIPT), "--candidate-version", "web-sync-test", "--source-dir", str(source), "--output", str(source / "bundle.tar")],
                cwd=ROOT,
                capture_output=True,
                text=True,
            )
        self.assertEqual(result.returncode, 3)
        self.assertIn("output must be outside the source directory", result.stderr)


if __name__ == "__main__":
    unittest.main()
