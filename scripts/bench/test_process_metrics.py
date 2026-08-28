import tempfile
import unittest
from pathlib import Path

from scripts.bench.process_metrics import capture_sample, summarize_samples


class ProcessMetricsTest(unittest.TestCase):
    def setUp(self):
        self.samples = [
            {
                "schema_version": "dipole.performance.process-sample.v1",
                "captured_monotonic_ns": 1_000_000_000,
                "clock_ticks_per_second": 100,
                "services": {
                    "gateway": {
                        "pid": 101,
                        "start_time_ticks": 500,
                        "cpu_ticks": 100,
                        "rss_bytes": 10_000,
                        "thread_count": 4,
                        "voluntary_context_switches": 20,
                        "involuntary_context_switches": 3,
                    }
                },
            },
            {
                "schema_version": "dipole.performance.process-sample.v1",
                "captured_monotonic_ns": 2_000_000_000,
                "clock_ticks_per_second": 100,
                "services": {
                    "gateway": {
                        "pid": 101,
                        "start_time_ticks": 500,
                        "cpu_ticks": 175,
                        "rss_bytes": 14_000,
                        "thread_count": 6,
                        "voluntary_context_switches": 28,
                        "involuntary_context_switches": 5,
                    }
                },
            },
            {
                "schema_version": "dipole.performance.process-sample.v1",
                "captured_monotonic_ns": 3_000_000_000,
                "clock_ticks_per_second": 100,
                "services": {
                    "gateway": {
                        "pid": 101,
                        "start_time_ticks": 500,
                        "cpu_ticks": 220,
                        "rss_bytes": 12_000,
                        "thread_count": 5,
                        "voluntary_context_switches": 35,
                        "involuntary_context_switches": 9,
                    }
                },
            },
        ]

    def test_summarizes_cpu_memory_threads_and_context_switches(self):
        report = summarize_samples(self.samples)

        self.assertEqual(report["schema_version"], "dipole.performance.process-resources.v1")
        self.assertEqual(report["sample_count"], 3)
        self.assertEqual(report["duration_seconds"], 2.0)
        self.assertEqual(
            report["services"]["gateway"],
            {
                "pid": 101,
                "cpu_core_percent": 60.0,
                "rss_start_bytes": 10_000,
                "rss_end_bytes": 12_000,
                "rss_peak_bytes": 14_000,
                "thread_peak": 6,
                "voluntary_context_switches": 15,
                "involuntary_context_switches": 6,
            },
        )

    def test_captures_linux_proc_process_and_thread_counters(self):
        with tempfile.TemporaryDirectory() as directory:
            proc_root = Path(directory)
            process_root = proc_root / "101"
            task_root = process_root / "task" / "101"
            task_root.mkdir(parents=True)
            stat_fields = ["S"] + ["0"] * 19
            stat_fields[11] = "7"
            stat_fields[12] = "3"
            stat_fields[19] = "500"
            (process_root / "stat").write_text(
                "101 (gateway worker) " + " ".join(stat_fields) + "\n",
                encoding="utf-8",
            )
            (process_root / "status").write_text(
                "VmRSS:\t12 kB\nThreads:\t1\n",
                encoding="utf-8",
            )
            (task_root / "status").write_text(
                "voluntary_ctxt_switches:\t8\nnonvoluntary_ctxt_switches:\t2\n",
                encoding="utf-8",
            )

            sample = capture_sample({"gateway": 101}, proc_root)

        process = sample["services"]["gateway"]
        self.assertEqual(process["start_time_ticks"], 500)
        self.assertEqual(process["cpu_ticks"], 10)
        self.assertEqual(process["rss_bytes"], 12 * 1024)
        self.assertEqual(process["thread_count"], 1)
        self.assertEqual(process["voluntary_context_switches"], 8)
        self.assertEqual(process["involuntary_context_switches"], 2)

    def test_requires_at_least_two_samples(self):
        with self.assertRaisesRegex(ValueError, "at least two"):
            summarize_samples(self.samples[:1])

    def test_rejects_service_set_drift(self):
        self.samples[1]["services"]["core"] = self.samples[1]["services"]["gateway"].copy()

        with self.assertRaisesRegex(ValueError, "service set"):
            summarize_samples(self.samples)

    def test_rejects_process_restart(self):
        self.samples[2]["services"]["gateway"]["pid"] = 202

        with self.assertRaisesRegex(ValueError, "process identity"):
            summarize_samples(self.samples)

    def test_rejects_counter_regression(self):
        self.samples[2]["services"]["gateway"]["cpu_ticks"] = 99

        with self.assertRaisesRegex(ValueError, "cpu_ticks regressed"):
            summarize_samples(self.samples)


if __name__ == "__main__":
    unittest.main()
