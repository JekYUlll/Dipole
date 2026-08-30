import stat
import subprocess
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "smoke-sync-write-ownership.sh"


class SyncOwnershipSmokeContractTest(unittest.TestCase):
    def test_script_is_executable_and_shell_valid(self):
        self.assertTrue(SCRIPT.stat().st_mode & stat.S_IXUSR)
        subprocess.run(["bash", "-n", str(SCRIPT)], check=True)

    def test_receipt_is_explicitly_safe_and_rollback_bound(self):
        source = SCRIPT.read_text(encoding="utf-8")
        for required in (
            "SMOKE_REPORT_FILE",
            "dipole.sync.write-ownership-smoke-receipt.v1",
            "source_revision",
            "source_dirty",
            "projector_write:true",
            "atomic_rollback:true",
            "destructive_data_migration:false",
            "chmod 0600",
            "mv -f",
        ):
            self.assertIn(required, source)


if __name__ == "__main__":
    unittest.main()
