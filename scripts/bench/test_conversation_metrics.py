import unittest

from scripts.bench.conversation_metrics import build_delta


def snapshot(direct_writes, direct_success_count, direct_success_sum, direct_buckets):
    lines = [
        f'dipole_conversation_projection_writes_total{{projection="direct_message"}} {direct_writes}',
        'dipole_conversation_projection_writes_total{projection="group_init"} 0',
        'dipole_conversation_projection_writes_total{projection="group_message"} 0',
        f'dipole_conversation_projection_write_duration_seconds_count{{outcome="success",projection="direct_message"}} {direct_success_count}',
        f'dipole_conversation_projection_write_duration_seconds_sum{{outcome="success",projection="direct_message"}} {direct_success_sum}',
        'dipole_conversation_projection_write_duration_seconds_count{outcome="error",projection="direct_message"} 0',
        'dipole_conversation_projection_write_duration_seconds_sum{outcome="error",projection="direct_message"} 0',
    ]
    for upper_bound, count in direct_buckets:
        lines.append(
            'dipole_conversation_projection_write_duration_seconds_bucket'
            f'{{le="{upper_bound}",outcome="success",projection="direct_message"}} {count}'
        )
    for projection in ("group_init", "group_message"):
        for outcome in ("success", "error"):
            lines.extend([
                'dipole_conversation_projection_write_duration_seconds_count'
                f'{{outcome="{outcome}",projection="{projection}"}} 0',
                'dipole_conversation_projection_write_duration_seconds_sum'
                f'{{outcome="{outcome}",projection="{projection}"}} 0',
                'dipole_conversation_projection_write_duration_seconds_bucket'
                f'{{le="+Inf",outcome="{outcome}",projection="{projection}"}} 0',
            ])
    return "\n".join(lines) + "\n"


class ConversationMetricsTest(unittest.TestCase):
    def test_aggregates_nodes_and_calculates_average_and_p95_bound(self):
        before = [
            snapshot(10, 10, 0.10, [("0.01", 8), ("0.025", 10), ("+Inf", 10)]),
            snapshot(20, 20, 0.20, [("0.01", 15), ("0.025", 20), ("+Inf", 20)]),
        ]
        after = [
            snapshot(12, 12, 0.13, [("0.01", 9), ("0.025", 12), ("+Inf", 12)]),
            snapshot(23, 23, 0.26, [("0.01", 16), ("0.025", 23), ("+Inf", 23)]),
        ]

        result = build_delta(before, after)

        self.assertEqual(result["projection_writes"]["direct_message"], 5)
        self.assertEqual(result["write_operations"], 5)
        timing = result["timing"]["direct_message"]
        self.assertEqual(timing["success_count"], 5)
        self.assertEqual(timing["error_count"], 0)
        self.assertAlmostEqual(timing["average_success_ms"], 18.0)
        self.assertEqual(timing["p95_success_upper_bound_ms"], 25.0)

    def test_rejects_counter_reset(self):
        before = [snapshot(10, 10, 0.10, [("+Inf", 10)])]
        after = [snapshot(9, 9, 0.09, [("+Inf", 9)])]

        with self.assertRaisesRegex(ValueError, "reset"):
            build_delta(before, after)

    def test_rejects_success_counter_mismatch(self):
        before = [snapshot(10, 10, 0.10, [("+Inf", 10)])]
        after = [snapshot(12, 11, 0.12, [("+Inf", 11)])]

        with self.assertRaisesRegex(ValueError, "successful duration count"):
            build_delta(before, after)


if __name__ == "__main__":
    unittest.main()
