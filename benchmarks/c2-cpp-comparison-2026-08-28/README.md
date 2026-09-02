# C2 Go/C++ Realtime Comparison Evidence

本目录绑定同一 clean revision 下的 Go 实时投递基线、C++ Realtime Delivery shadow 投影和真实 Gateway queue saturation 测试。

## 结果

- Go 与 C++ candidate 均绑定 `a2dfd72a07456b086b0ef140c7a0b0df32aaaebc`，镜像标签均为 `dirty=false`。
- `concurrent` workload 使用 20 个用户，每个用户发送 2 条文本消息。Go 共 attempted/accepted/persisted/received 40/40/40/40，expected receipts 为 40，Kafka settled lag 为 0。
- C++ 独立 consumer group 观察 80 个 Kafka 坐标；`message_type=0` selector 选中 40 条 workload 消息，另外 40 条好友初始化系统消息保持在原始 evidence 中并计为 filtered-out。
- 选中的 40 条记录全部最终 projected，node transport requested/observed 为 40/40，最终 duplicate/rejected/backpressured 均为 0，比较报告决策为 `eligible`。
- Go race 测试通过真实 TCP/mTLS listener 将容量为 1 的 observation queue 饱和，并验证第三个批次返回 `BACKPRESSURED/QUEUE_FULL`；C++ node transport 与 ShadowRunner 测试同时通过。
- 演练前显式确认 Go Gateway 的 `dipole.message.direct.created` 六个分区和 C++ 的十二个 direct/group 分区均已 assignment，且 workload 前 direct topic log end 为 0。

## 发现的边界

- 空 Kafka 拓扑首次启动时，Go Gateway 曾在 HTTP readiness 为 200 的情况下没有形成 `dipole-gateway-consumer` assignment；该轮 40 条消息已持久化但实时收件为 0。重启 Gateway 后 assignment 恢复。此风险记录为 `AD-039`，本归档只使用满足显式 assignment 前置条件的成功运行。
- C++ observation receiver 仍为默认关闭的 shadow sink，不写 WebSocket 客户端。对照证明相同 Kafka workload 下的路由与节点接纳一致性，尚未证明 C++ 已具备客户端投递 ACK 语义。

## 文件

- `report.json`：符合 comparison v1 schema 的 `eligible` 报告及输入 SHA-256。
- `runtime-provenance.json`：C++ runtime 的不可变 container/image/revision 绑定。
- `raw/go-baseline.json`：Go baseline v4。
- `raw/go-runtime-provenance.json`：Go Core/Message/Sync/Gateway 镜像来源。
- `raw/cpp-shadow-evidence.ndjson`：完整低敏 v3 shadow evidence，不含正文、用户、recipient、connection、密钥或证书。
- `raw/cpp-group-final.txt`、`raw/go-gateway-group-final.txt`：最终 consumer group offset 与 lag。
- `raw/backpressure-go.txt`、`raw/backpressure-cpp.txt`：真实 Go TCP queue saturation 与 C++ transport/runner 测试输出。
- `SHA256SUMS`：归档完整性校验。

在本目录运行 `sha256sum --check SHA256SUMS` 可验证归档。
