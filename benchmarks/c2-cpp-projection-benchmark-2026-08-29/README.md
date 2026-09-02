# C2 Go/C++ Projection Performance Evidence

本目录记录同一份 `message.direct.created` v1 事件、同一迭代量和同一机器上的 Go/C++ projection microbenchmark。Go 每次循环执行 JSON 解码和 Sync projection；C++ 每次循环执行 JSON 解码、事件校验和 Delivery envelope projection。报告只用于数据面替换门禁，不代表完整 Gateway 或端到端吞吐。

## 结果

- 100,000 次迭代的结果计数一致。
- C++/Go projection ops ratio 约为 `0.10`，低于默认 `1.0` 晋级门槛，因此报告为 `blocked`。
- 当前结论是保留 Go 投影实现，停止将 C++ projection 晋级为默认实现；C++ 的故障隔离、Primary authority 和未来连接/批处理数据面评估继续独立保留。

## 运行

```bash
DIPOLE_GRPC_ROOT=/tmp/dipole-grpc-root \
DIPOLE_RDKAFKA_ROOT=/tmp/dipole-rdkafka-root \
DIPOLE_CPP_BUILD_DIR=/tmp/dipole-cpp-realtime-benchmark \
./scripts/bench/realtime_projection_benchmark.sh
```

可通过 `DIPOLE_REALTIME_BENCH_ITERATIONS` 调整样本量，通过 `DIPOLE_REALTIME_BENCH_MIN_RATIO` 调整晋级门槛。报告中的 `blocked` 是 fail-closed 结果，不改变默认 Go authority。

## 文件

- `report.json`：机器可读的双语言结果、比例和晋级判定。
