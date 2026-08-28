# C2 Go/C++ Projection Performance Evidence Rerun

本目录记录 2026-08-29 在当前候选 revision 上重新执行的同 workload Go/C++ projection microbenchmark。测试使用 `message.direct.created` v1 事件和 100,000 次迭代，报告只用于数据面替换门禁，不代表完整 Gateway 或端到端吞吐。

## 结果

- Go/C++ 结果计数均为 100,000，计数一致。
- C++/Go projection ops ratio 为 `0.0980914929`，低于默认 `1.0` 晋级门槛，判定为 `blocked`。
- 继续保留 Go projection；C++ 故障隔离、Primary authority 和连接/批处理数据面评估保持独立。

## 复现

```bash
DIPOLE_REALTIME_BENCH_OUTPUT=/tmp/dipole-cpp-projection-latest.json \
DIPOLE_REALTIME_BENCH_ITERATIONS=100000 \
./scripts/bench/realtime_projection_benchmark.sh
```

报告中的 `blocked` 是 fail-closed 结果，不改变默认 Go authority。

## 文件

- `report.json`：机器可读的双语言结果、比例和晋级判定。
