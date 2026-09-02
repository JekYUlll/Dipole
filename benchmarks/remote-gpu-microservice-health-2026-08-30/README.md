# Remote GPU Microservice Health Probe

这是 Remote GPU 开发环境中微服务 Gateway 入口的只读稳定性探针，不代表消息吞吐容量，也不替代 C1 k6 基线。

## Evidence

- Revision: `f227401a`
- Topology: isolated `docker-compose.microservices.yml` project
- Workload: `GET /health`, 1000 requests, concurrency 16, timeout 5 seconds
- Result: 1000 successes, 0 failures
- Latency: P50 `0.000521s`, P95 `0.000791s`, P99 `0.001960s`
- Services: MySQL, Redis, Kafka, MinIO, Core, Message, Sync and Gateway reached healthy/readiness
- Cleanup: the isolated project had no remaining containers or volumes after the run
- Safety: existing GPU workloads and unrelated projects were not modified

## Boundary

The complete `scripts/bench/run_bench.sh` workload still requires the legacy three-node candidate topology, a revision-matched `dipole-server` image and a remote `k6` binary. This probe must not be used for throughput, WebSocket delivery, Kafka lag or fan-out claims.
