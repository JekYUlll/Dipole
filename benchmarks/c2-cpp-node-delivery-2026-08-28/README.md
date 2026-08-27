# C2 C++ Node Delivery Shadow Evidence

本目录绑定 C++ Realtime Delivery、Kafka、Redis Presence 与 Go Gateway 的首个真实跨进程 node batch shadow 演练。

## 结果

- C++ candidate `4105d2159af0773bd8360ca4e605c0c5c6d9a7db` 从独立 consumer group 读取 direct created event，经 Presence 将一个在线连接聚合为 `gateway-1` 节点批次。
- C++ 以 `dipole-realtime` mTLS 身份调用 Go Gateway `NodeDeliveryService`；Gateway 镜像绑定 `d68947c8c23a83d231b13d21a91be1725d61531c`，接收端监听 `0.0.0.0:9095`。
- 首条合法事件产生 `transport_requested=1`、`transport_observed=1`，Gateway shadow sink 不持有 Hub 或 Client，因此客户端写入为 0。
- Gateway 不可用时，事件记录为 `deferred/node_transport`，partition offset 保持未提交；Gateway 恢复并重启 worker 后，同一 record 被重新读取、观察并提交，最终 group lag 为 0。
- 将已提交 offset 回拨一条并保持 Gateway 进程不变后，稳定 `batch_id` 命中去重，证据记录 `transport_duplicate=1`。
- 一条刻意注入的无效事件产生固定 `invalid_event`，完整归档保留调试期间的全部 10 条低敏记录，没有消息正文、recipient、密钥或证书。
- 演练完成后，隔离 Compose 项目的容器、网络和数据卷均已移除，共享开发拓扑未参与清理。

## 已知边界

- 当前 worker 在单进程内不会 seek 并重试 deferred record；恢复依靠进程重启后从 committed offset 重放。后续需增加有界退避和 partition pause/resume。
- 根 Dockerfile 复制预构建 `dist/`。本次发现仅更新镜像 revision 标签不足以证明二进制新鲜度，最终 Gateway 镜像已在执行 `scripts/docker-build.sh backend` 后重新构建。
- Gateway receiver 仍为默认关闭的 observation sink，不向 WebSocket 客户端投递，也未接管 Go Delivery。

## 文件

- `report.json`：机器可读的 revision、镜像、mTLS、恢复和已知边界汇总。
- `raw/shadow-evidence.ndjson`：完整低敏 v3 逐 record 证据。
- `raw/group-final.txt`：演练结束时的 consumer group offset 与 lag。
- `SHA256SUMS`：归档完整性校验。

在本目录运行 `sha256sum --check SHA256SUMS` 可验证归档。
