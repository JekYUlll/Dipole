# C3 Go/C++ Projection Benchmark - 2026-08-31

## Result

The Remote GPU development host ran the same `message.direct.created` v1
projection workload for 100,000 iterations at revision
`bed7a5d06f5f69bbccc0de4586235881e5b6d5ae`.

| Field | Result |
| --- | --- |
| C++ runner | Ubuntu 24.04 Dockerfile builder |
| C++ CTest | 14/14 passed |
| C++ throughput | 31,185.72 ops/s |
| Go throughput | 129,964.51 ops/s |
| C++ / Go ratio | 0.239956 |
| Eligibility threshold | 1.0 |
| Decision | `blocked` |

The event item counts are both 100,000. The workload remains a projection
microbenchmark, so it does not represent end-to-end Gateway throughput.

## Boundary

The fail-closed result keeps Go as the delivery authority. It does not change
the C++ shadow, fencing, rollback, or future connection/batching evaluation
paths. The benchmark used an isolated build container and left no running
Dipole container after completion.

## Reproduction

```bash
DIPOLE_GO_BIN=/home/admin1/.local/go-1.27.0/bin/go \
DIPOLE_REALTIME_BENCH_CONTAINER=1 \
DIPOLE_REALTIME_BENCH_OUTPUT=/tmp/dipole-cpp-projection-latest.json \
scripts/bench/realtime_projection_benchmark.sh
```

See `report.json` for the machine-readable result.
