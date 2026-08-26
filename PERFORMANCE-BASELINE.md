# Performance Baseline

本文档滚动记录可重复的性能基线。微基准用于比较架构演进前后的协议与实现开销，完整发送链路、Kafka lag、Inbox 写放大和热群 fanout 继续由 G0 端到端压测覆盖。

## 2026-08-26：Message History Transport

环境：

```text
CPU: AMD Ryzen 7 8845H
OS/Arch: linux/amd64
Transport: loopback TCP
Security: TLS 1.3 mTLS + service secret + certificate caller binding
Payload: one direct-history Message
```

命令：

```bash
LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu \
go test ./internal/bootstrap \
  -run '^$' \
  -bench '^BenchmarkMessageTransportDirectHistory$' \
  -benchmem -benchtime=2s -count=3
```

结果：

| Transport | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Local | 50.28-50.72 | 296 | 2 |
| gRPC mTLS | 69,339-69,618 | 14,932-14,934 | 243 |

M4 loopback adapter 门槛为平均 `<1 ms/op`，本次三轮 Remote 结果约 `0.0695 ms/op`，通过门槛。该结果不代表公网或跨节点端到端 P95/P99；后续 C++ Gateway/Delivery 对比必须复用独立的连接数、吞吐、P50/P95/P99、CPU 与 RSS 基线。
