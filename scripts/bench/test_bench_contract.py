import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parent


class BenchContractTest(unittest.TestCase):
    def test_report_uses_selected_scenario_filter(self):
        source = (ROOT / "run_bench.sh").read_text(encoding="utf-8")
        self.assertIn('REPORT_SCENARIO="${SCENARIO_FILTER:-${SCENARIO}}"', source)
        self.assertIn('--arg scenario "${REPORT_SCENARIO}"', source)

    def test_group_window_is_configurable_and_longer_than_legacy_default(self):
        source = (ROOT / "bench.js").read_text(encoding="utf-8")
        self.assertIn('const GROUP_MAX_DURATION = __ENV.GROUP_MAX_DURATION || "35s";', source)
        group_block = source.split("group_blast:", 1)[1].split("},", 1)[0]
        self.assertIn("maxDuration: GROUP_MAX_DURATION", group_block)
        self.assertNotIn('maxDuration: "20s"', group_block)


if __name__ == "__main__":
    unittest.main()
