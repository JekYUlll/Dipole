# Cassandra Timeline Reader Baseline

## Scope

This receipt compares the SQLC/MySQL and Cassandra implementations of
`ConversationTimelineReader` using one isolated conversation and the same
closed Seq range. It is a throughput-oriented Go benchmark, not a production
latency or P99 result.

## Command

```bash
DIPOLE_TIMELINE_BENCH_MESSAGES=100 \
DIPOLE_TIMELINE_BENCH_TIME=1s \
scripts/smoke-cassandra-read-routing.sh
```

The script starts temporary MySQL and Cassandra containers, applies migration
`000058` and the Timeline schema, runs the read-routing fallback contract, then
benchmarks `[1,100]`. Fixture writes, connection setup and cleanup occur before
or after the timed loop.

## Environment

| Field | Value |
| --- | --- |
| Revision | `f653af2d` |
| Host CPU | Intel Xeon Platinum 8481C |
| Go target | linux/amd64 |
| Fixture | 100 contiguous messages in one conversation |
| Sample duration | 1 second per reader |

## Results

| Reader | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| SQLC/MySQL | 696874 | 192808 | 1953 |
| Cassandra | 2391275 | 227071 | 1170 |

The same run verified Cassandra primary page reads plus MySQL fallback after a
payload mismatch and a missing Timeline row. This single-sample baseline only
guides follow-up tuning; repeated hardware-controlled samples, concurrent load,
page-size sweeps and P50/P95/P99 collection remain required before making
performance claims.
