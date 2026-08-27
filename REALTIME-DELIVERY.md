# Realtime Delivery

本文档定义 Go Gateway 与后续 C++ Realtime Delivery 共享的 v1 投递边界。当前生产流量继续经过 Go Kafka consumer、Redis Presence/PubSub 和 WebSocket Hub；本里程碑只冻结协议与兼容适配层。

## 当前链路

```text
message.direct.created / message.group.created
                    |
                    v
             Go Kafka handler
                    |
       +------------+-------------+
       |                          |
       v                          v
 full/timeline event       hot-group notify
       |                    200 ms aggregate
       +------------+-------------+
                    |
             Redis Presence
                    |
          local Hub / node PubSub
                    |
             WebSocket queue
```

普通群当前按收件人启动 goroutine；热群仍按收件人解析路由，但在窗口内合并为最新 Seq 通知并由客户端补拉。Redis Pub/Sub 保持 at-most-once，Sync Timeline 负责最终恢复。

## v1 契约

权威定义位于 `api/proto/dipole/delivery/v1/delivery.proto`：

- `DeliveryEnvelope` 保存 Kafka topic/partition/offset、事件与 trace 绑定，以及待路由的用户级投递项。
- `NodeDeliveryBatch` 是 Presence 解析后的节点批次；每批只指向一个 Gateway node，每项只携带该节点拥有的 connection ID。
- `delivery_id` 在重试中保持稳定，`ordering_key` 固定同一用户或会话的执行顺序。
- `DeliveryMode` 区分完整事件、Timeline 通知和热群通知，避免 C++ 实现重新推断业务策略。
- `DeliveryAck` 逐项返回 `ENQUEUED|OFFLINE|BACKPRESSURED|REJECTED|FAILED`；`OFFLINE` 表示路由已完成，客户端依靠 Sync 恢复。
- `BACKPRESSURED` 必须同时给出逐项与饱和队列的 `retry_after_ms`。每个 envelope/node batch 最多 4096 项，超限由生产者拆分。

三个 Protobuf JSON golden vectors 分别固定用户级 envelope、节点批次和背压 ACK。Go validation 拒绝未知枚举、负 Kafka 坐标、无效时间戳、重复 delivery/connection ID 和不一致 ACK 状态。

## C++ contract-only foundation

`realtime-delivery/` 提供独立 C++20 CMake target。构建过程直接读取 canonical Proto 并把 C++ 生成物写入 build 目录；`contract_validator` 与 Go 读取同三组 golden vectors。统一入口 `scripts/check-cpp-realtime.sh` 固定系统编译器和 Protobuf ABI，执行 warnings-as-errors、clang-tidy 与 CTest。

foundation 只接受 `DIPOLE_REALTIME_MODE=contract_only`。`serve` 在启动 listener 前验证完整 golden directory，随后暴露 `/livez`、`/readyz`、`/health`；host 仅允许 `0.0.0.0|127.0.0.1`，port 仅允许 `1..65535`。它没有 Kafka、Redis、Gateway transport 或业务存储依赖，也没有进入 Compose。

## C++ 消息事件纯投影

`event_projection` 将 broker adapter 提供的 `{topic, partition, offset, value}` 与显式策略输入转换为 `DeliveryEnvelope`，函数内部没有网络、Redis、时钟或进程级状态。Kafka 重放同一 record 会生成相同 batch ID、delivery ID、排序键、事件时间与 payload。

当前只接受 `message.direct.created` 和 `message.group.created` 的 v1/minor-additive envelope，并保持现有 Go Delivery 语义：

- 私聊向 target 生成 `chat.message`，普通群按原 recipient 顺序排除 sender。
- `timeline_notify_shadow` 为每个完整消息追加 `sync.item.notify.v1`；legacy created 缺少 Seq 时仍可投递完整消息，但拒绝生成 Timeline 通知。
- `hot_group` 生成 `group.message.notify`，沿用包含 sender 的完整 recipient 集合，并抑制逐消息 Timeline 通知。
- 文件消息生成与 Go `FilePayload` 一致的下载路径和可选过期时间；事件未知字段继续允许 producer-first 发布。
- channel/target 不匹配、未知 major version、无效时间戳、重复 recipient 和空群 fanout 均 fail closed；输出返回前再次执行 Delivery v1 validator。

该投影尚未被 `serve` 调用。后续 librdkafka adapter 只负责 poll、record ownership、重平衡和 commit；Redis 热群分类与对照证据也保持在投影之外，避免 broker 生命周期改变业务映射。

## Kafka shadow 消费边界

`LibrdkafkaConsumer` 固定订阅 `dipole.message.direct.created` 与 `dipole.message.group.created`，group ID 必须使用 `dipole-realtime-shadow-*` 前缀。配置强制 `earliest`、关闭 auto commit/offset store、使用 round-robin assignment，并以 rebalance callback 的实际 partition 数作为 readiness 输入。poll 后立即复制 record，销毁 librdkafka message ownership；同步 commit 明确提交当前 `offset + 1`。

`ShadowRunner` 的顺序固定为 `poll -> project/reject -> append evidence -> commit`。NDJSON evidence v1 只保存 topic/partition/offset、event/batch ID、item count、projected/rejected 和固定错误类别，不保存 recipient、消息正文或原始异常。evidence 写失败与 commit 失败都会撤销 runner readiness；poison event 在记录 `invalid_event` 后允许独立 shadow group 前进。

`shadow` 命令已组合这些组件。consumer worker 独占 Kafka 和 evidence stream，主线程暴露健康面；`/livez` 始终报告进程存活，`/readyz` 和 `/health` 仅在实际 assignment 非空且最近操作健康时返回 200。SIGINT/SIGTERM 会停止 worker 并关闭 consumer，不进入 Gateway 写路径。

无 Redis 阶段使用 created event 中持久化的 `sync_fanout=false` 选择热群通知模式，避免历史回放误做完整 fanout；动态 `recent_message_count` 暂以 0 表达未知，后续 Redis adapter 独立补齐。该字段不参与当前低敏 evidence。

## Offset 与重试边界

现有 Go consumer 在 handler 成功返回后提交 Kafka offset，但 Redis `PUBLISH` 和本地 `Client.Enqueue` 没有持久 ACK。v1 legacy adapter 只将当前返回值映射为 `ENQUEUED/OFFLINE`，不改变该语义。

C++ shadow 阶段遵守以下门禁：

1. 使用独立 consumer group，只生成 route/batch/ACK 对比证据，不向客户端投递。
2. 对比 source event、目标节点、收件人、connection、mode、ordering key 和处理时延。
3. 有界队列满时返回 `BACKPRESSURED`，不得静默丢弃；Kafka offset 策略在 primary 切流前单独故障演练。
4. primary 阶段提交 offset 前必须收到节点 ACK 或写入可重放的持久边界；稳定 delivery ID 配合 Gateway 去重后才能开启自动重试。
5. `OFFLINE` 可提交，消息事实与 Inbox 已持久化；客户端重连后从 Sync Timeline 恢复。

当前主机具备 nlohmann/json 3.11.3；librdkafka 2.3.0 通过 Ubuntu Noble 包无特权解压到临时 sysroot 完成编译与测试，未修改系统包。发布构建必须把 librdkafka 版本写入构建镜像和运行证据，禁止依赖开发机隐式库。

## 进程与数据所有权

C++ Delivery 可读取 Kafka 事件与 Redis Presence/热点状态，可向 Gateway 节点发送批次并输出指标。它不得连接 MySQL、Cassandra、Elasticsearch、Agent Runtime 或对象存储，也不得重新执行成员权限、消息持久化和 Conversation/Inbox 投影。

Gateway 继续拥有连接认证、心跳、WebSocket envelope、连接级有界队列和客户端写入。C++ Delivery 达到 shadow 门禁后再评估节点级传输；Gateway 替换属于后续独立里程碑。

## 回滚

路线图预留 `realtime.delivery=go|shadow|cpp` 开关语义；当前生产配置和 Compose 尚未加入该开关，系统等价于 `go`。独立执行 `shadow` 只增加观察链，失败不会阻塞 Go；`cpp` 需要节点投递、自动回切和同一 workload 的 Go/C++ 对照证据。任何 ACK 漂移、队列溢出、顺序差异或恢复退化都回切 `go`，无需数据回滚。
