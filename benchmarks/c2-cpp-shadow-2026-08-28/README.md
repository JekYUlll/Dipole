# C2 C++ Kafka Shadow Evidence

本目录记录 `ef763a4b9fa090b9ba14c1f43e78ca723f9e2ef6` 在现有 `dipole-kafka` Kafka 3.9.0 broker 上的 shadow-only 演练。C++ 使用无特权解压的 Ubuntu Noble librdkafka 2.3.0，未安装系统包，未启动 Redis/Gateway adapter，也未向客户端写入。

## 结果

- 唯一 group `dipole-realtime-shadow-live-v2` 以 earliest 回放 canonical direct/group created topics。
- broker 当时 direct topic 六个 partition 的 log end 均为 0；group topic 共回放 205 条合法事件，全部生成 projected evidence。
- 向 group topic 注入一条无效 JSON 后，生成一条 `invalid_event` evidence，随后 partition 3 committed offset 到 40，lag 为 0。
- 单实例 ready 为 200。第二实例加入相同 group 后，两者 ready 均为 200，并分担总计 12 个 topic partition。
- 停止第一实例后，第二实例接管全部 12 个 partition并保持 ready=200；最终优雅停止后，六个有数据 group partition lag 全为 0。
- NDJSON 不含消息正文、recipient 或原始异常，只包含 Kafka coordinates、稳定 event/batch ID、item count 和固定 outcome/error code。

## 文件

- `report.json`：机器可读汇总及 commit/依赖/group 绑定。
- `raw/shadow-evidence.ndjson`：206 条低敏逐 record evidence。
- `raw/group-final.txt`：全部实例停止后的 committed offset 与最终 lag。
- `SHA256SUMS`：归档文件完整性。

该证据覆盖 earliest replay、poison 隔离、双实例 rebalance、单实例接管与 readiness。direct topic 当时没有 retained record，direct 的真实 broker payload 仍需后续同 workload 对照；Redis Presence、节点批次、队列背压和 Gateway ACK 属于后续 C2 门禁。
