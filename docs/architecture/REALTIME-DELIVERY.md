# Realtime Delivery

本文档定义 Go Gateway 与 C++ Realtime Delivery 共享的 v1 投递边界。当前生产流量继续经过 Go Kafka consumer、Redis Presence/PubSub 和 WebSocket Hub；C++ 提供默认关闭的 observation shadow 和候选 primary CLI，尚未进入 Compose 或接管生产流量。

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

`services/realtime-delivery/` 提供独立 C++20 CMake target。构建过程直接读取 canonical Proto 并把 C++ 生成物写入 build 目录；`contract_validator` 与 Go 读取同三组 golden vectors。统一入口 `scripts/check-cpp-realtime.sh` 固定系统编译器和 Protobuf ABI，执行 warnings-as-errors、clang-tidy 与 CTest。

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

Presence 路由先落为纯投影：输入已解析的用户连接快照与明确的 `now/ttl`，过滤过期连接后按 node 稳定排序生成 `NodeDeliveryBatch`，并统计 observed、eligible、stale 与 offline。用户身份漂移、空 node/connection 和重复 connection 所有权会 fail closed。ShadowRunner 已通过 hiredis adapter 读取 Go 原始 Hash，并以 TTL 过滤视图生成节点批次；该差异继续保留在低敏 evidence 中。

hiredis adapter 只执行 Presence `HGETALL`，支持 direct 与 Sentinel master discovery、Redis/Sentinel 密码、DB 选择和批量 pipeline。网络、协议或认证失败会关闭连接，使下一轮重新发现 master；坏 Hash 记录按计数隔离。adapter 已注入 Kafka ShadowRunner，并通过真实单节点 Redis fixture 与三 Redis/三 Sentinel 切主演练。

`DIPOLE_REALTIME_PRESENCE_MODE=shadow` 会将 reader 注入 Kafka runner；direct 使用 `DIPOLE_REALTIME_REDIS_ENDPOINT`，Sentinel 使用 `DIPOLE_REALTIME_REDIS_SENTINELS` 与 `DIPOLE_REALTIME_REDIS_MASTER_NAME`。证据格式升级为 `dipole.realtime.shadow-evidence.v2`，只增加节点批次数和 Presence 聚合计数。Redis 读取失败不写 evidence、不提交 offset并撤销 readiness；可读取但身份/所有权冲突记录固定 `invalid_presence` 后允许独立 shadow group 前进。

首轮 Kafka+Redis 联合回放消费 206 条 retained 记录：205 projected、1 个既有 poison rejected；隔离 fixture 在 20 条消息中产生 20 个节点批次、20 eligible 与 20 stale，malformed 为 0，最终 group lag 为 0。fixture 与进程已清理。

Sentinel 恢复证据位于 `benchmarks/c2-cpp-presence-2026-08-28/`。隔离的三 Redis/三 Sentinel 拓扑中，同一 reader 在停止当前 master `redis-2` 后连续执行 80 次读取，记录 5 次有界错误并完成 75 次成功读取，随后自动发现 `redis-3`，进程未重启。测试 Compose、网络、fixture 和卷均已删除。

## Offset 与重试边界

节点 transport shadow 使用 canonical gRPC `NodeDeliveryService.ObserveNodeBatch`。返回的 `NodeDeliveryObservation` 只证明 Gateway 节点验证并接纳了观察任务，可表达 `OBSERVED`、`REJECTED`、`BACKPRESSURED` 和稳定 batch 去重；它不声明 WebSocket queue 已入队。真实客户端投递继续使用独立 `DeliveryAck` 语义，后续 promotion 前再增加对应 RPC。

Gateway 已提供默认关闭的 observation receiver。启用时，它在独立 listener 上只接受共享密钥或 mTLS 认证后的 `dipole-realtime`，要求批次目标与本机 Redis Presence node ID 一致，并将 protobuf clone 放入有界内存队列。receiver 的 sink 只累计批次、条目和 connection 数，不持有 Hub/Client 引用；重复 batch 返回 `duplicate=true`，队列满返回容量、深度和重试提示。关闭流程先停止 RPC 接入，再排空观察队列。微服务 Compose 已提供默认 `false` 的显式开关和 Realtime 证书身份，生产流量仍不启用。

C++ transport 通过 `DIPOLE_REALTIME_NODE_TRANSPORT_MODE=shadow` 显式启用，并要求 Presence shadow 同时开启。`DIPOLE_REALTIME_NODE_TARGETS` 使用逗号分隔的 `node_id=host:port` 精确映射；共享服务密钥由 `DIPOLE_INTERNAL_RPC_SHARED_SECRET` 注入，每次调用传播 batch 中已有 request/trace ID 并使用有界 deadline。明文只允许 loopback；跨容器或跨主机必须配置 CA、Realtime 客户端证书、私钥和 server name 形成 mTLS。

证据格式随 transport 扩展为 `dipole.realtime.shadow-evidence.v3`，增加低敏 `message_type` selector 及 requested/observed/duplicate/rejected/backpressured 聚合计数，不保存正文、收件人或 connection ID。处理顺序固定为 `poll -> project -> Presence -> node observation -> evidence -> commit`。节点 RPC 故障、拒绝或背压会写 `outcome=deferred,error_code=node_transport`，随后保持 offset 未提交并撤销 readiness；部分节点已接纳时，重放沿用稳定 batch ID，由 Gateway receiver 返回 duplicate，避免重复计数。Runner 现在保留至多一条未提交 record，并在现有有界错误退避后于同进程重试，commit 成功后才清除。

显式 `primary` 命令复用同一 record ownership，但授予独立的 `dipole-realtime-primary-*` consumer authority。启动必须同时设置 `DIPOLE_REALTIME_DELIVERY=cpp`、`DIPOLE_REALTIME_PRIMARY_ENABLED=true`、`DIPOLE_REALTIME_PRESENCE_MODE=primary` 和 `DIPOLE_REALTIME_NODE_TRANSPORT_MODE=primary`；缺少任一项都会在创建 consumer 前 fail closed。`shadow` 命令同样要求 `DIPOLE_REALTIME_DELIVERY=shadow`。tracked Compose 没有 C++ primary 服务或默认启用值。

primary 顺序固定为 `poll -> project -> Presence -> DeliverNodeBatch -> primary-evidence.v1 -> commit`。完整 ACK 必须与 batch 数量、batch ID 和全部 delivery ID 精确一致；所有结果均为 `ENQUEUED|OFFLINE` 时允许 commit，`PARTIAL/BACKPRESSURED`、`REJECTED`、`FAILED` 或任何漂移会记录有界分类并保留 pending record。Presence 已确认全部 item 离线且没有 node batch 时不发起 RPC，在 evidence 刷盘后直接提交。evidence append 仍先于 offset commit；进程在两者之间崩溃时依靠稳定 delivery ID、Gateway replay state、Web IndexedDB claim 和 Sync Timeline 补偿重放。

真实 primary runtime 证据位于 `benchmarks/c2-primary-runtime-2026-08-28/`。隔离环境中 terminal ACK 将目标分区提交到 log end；600 KiB probe 在第 40 个批次将真实 WebSocket queue 压至 16/16并收到 `PARTIAL/BACKPRESSURED`；错误 gRPC target worker 为同一坐标写 deferred/retain，`SIGKILL` 前 offset 保持、lag 为 1，正常 worker 重平衡后重放并将 lag 降为 0。报告 8/8 且绑定两张 clean same-revision 镜像。演练声明的是 deferred evidence 后崩溃重放，未声明窄 terminal evidence/commit 窗口的确定性故障点。

同一演练确认 Go Gateway consumer 与 C++ primary group 并行时会各写一条客户端 frame；legacy Go frame 没有可与 C++ stable delivery ID 合并的标识。`AD-041` 因此阻止 primary 加入 tracked Compose。C3 第一切片已加入默认 `go` 的本地 `realtime.delivery` 契约：`go|shadow` 使用 Go 消息投递 Handler，`cpp` 使用只校验消息事件并提交原 Go group offset 的 checkpoint Handler；非消息 Gateway 事件保持原行为。跨实例共享 authority fencing、双 group checkpoint receipt 和自动回切完成前仍禁止生产切流。

C3 本地 authority 证据位于 `benchmarks/c3-delivery-authority-2026-08-28/`。两个独立 Compose 项目使用相同 probe 契约：`go` 模式只收到一条 legacy frame，`cpp` 模式只收到一条带稳定 delivery ID 的 frame；Gateway 指标分别确认 `go`/`cpp` ownership。`cpp` 项目中原 Go group 以 checkpoint-only 路径消费，C++ primary group 在 terminal `ENQUEUED(1)` evidence 后提交，两个 group 的有记录分区均为 log end/lag 0。该结果完成本地互斥单帧门槛，未覆盖共享实例动态切换。

共享 fence v1 定义在 `contracts/realtime-delivery-fence/v1/`，Go/C++ 共同执行其 golden vectors。Redis 值以 `epoch` 防止旧进程在同 authority 回切后重新获得权限，以 `active|frozen` 建立显式无写入窗口，并用绝对 lease 截止时间约束控制面失联。Go Gateway 仅在 `realtime.fencing_enabled=true` 时装配 reader，要求部署提供精确非零 epoch；启动和每条 message-created 副作用前都会重新读取。任何不可验证状态都使 handler 停留在当前 Kafka record 上等待，直至 lease 恢复或进程上下文取消，避免 authority 暂停被现有 consumer 转移到业务 retry/DLQ。C++ 使用对应 `DIPOLE_REALTIME_FENCING_*` 配置，在 evidence/Kafka 初始化及每个 pending record 投影前核验；`shadow` 只接受共享 `shadow`，`primary` 只接受 `cpp`，拒绝会撤销 readiness 且保留坐标。

Go Gateway 使用 Presence `NodeID` 写入严格的 `observation.schema.json`。启动检查、独立 5 秒心跳和 dependency readiness 会刷新 15 秒 TTL，记录 expected/observed authority、epoch、phase、lease deadline、原始 lease SHA-256 与有界 reason code；Redis observation 写失败按拒绝处理。消息热路径继续使用只读 reader，避免每条消息追加 observation 写。

C++ shadow/primary 在 fencing 启用时要求显式稳定 `DIPOLE_REALTIME_INSTANCE_ID`，复用 hiredis 连接执行 `SET PX`。启动 observation 发生在 evidence 文件和 Kafka consumer 之前；runner 在空 poll 时调用五秒节流 heartbeat，在 pending record 上仍逐条读取 lease。denied 或 observation 写失败会撤销 fence readiness，保留 pending offset；成功 heartbeat 可在没有新消息时恢复 fence readiness，同时不会掩盖 evidence、commit 或 transport 的独立故障状态。共享 vectors 进一步固定未知 phase 与非正 lease 为 `invalid_record`，两端 reason code 一致。真实隔离 Redis 已观察到 authorized epoch 18、精确 lease hash 和约 15 秒 TTL 在 Kafka 不可用期间持续刷新。

控制面现在使用显式 expected-node manifest 聚合全部预期实例；Redis key 扫描不能代替该清单。每个节点可以声明本地 `expected_authority`，省略时兼容为 transition authority。聚合器逐键要求实际 TTL 存活、内部 5 至 60 秒生命期有效，并与 transition receipt 的 `next_sha256`、observed authority、epoch、phase 和 deadline 精确匹配。active proof 要求全部节点本地目标等于 active authority 且 authorized；frozen proof 允许清单同时包含准备中的 Go/shadow/C++ 节点，但每个节点都必须对同一 lease 报告 `denied/frozen`。双 group collector 随后以两次 read-committed log end 固定无写入采集窗口，核对 compatibility 与 primary group 的稳定状态、完整 assignment、逐分区零 lag 和一致高水位，并在 Kafka 采集结束后再次检查 observation/lease 尚未过期。DescribeGroups 使用原始协议响应并严格解析 ConsumerProtocol assignment v0-v3 的有界 topic/partition 前缀；其后的 client-owned opaque bytes 不参与 authority 判断，从而兼容 kafka-go v1 assignment 与 librdkafka v0 扩展，同时继续拒绝未知版本、截断、负值和重复 topic/partition。

`cmd/tools/realtime-cutover-checkpoint` 接受严格的 transition receipt、expected-node manifest 和 checkpoint manifest，成功后将完整 observation aggregate 与其哈希绑定的 Kafka receipt 写入 `checkpoint-bundle.schema.json`。输出文件权限为 `0600`，使用同目录临时文件、文件 fsync、不可覆盖 hard link 与目录 fsync 发布。该命令只生成 eligible receipt，不会改变 authority 或 offset；自动续切/回切及共享拓扑中断演练完成前，生产切流仍受 `AD-041` 阻断。

首个隔离实证归档于 `benchmarks/c3-cutover-checkpoint-2026-08-28/`。compatibility 侧使用真实 kafka-go member，候选侧使用当前 C++ librdkafka shadow member；两者在 direct/group 单分区上均提交到 `1/1`、lag 0，并生成 Schema 有效、mode `0600` 的 bundle。删除 expected-node observation 与停止候选 member 均 fail closed 且没有输出文件。该演练证明跨客户端 assignment 与 receipt 路径；候选进程仅以 shadow authority 建组，完整 frozen/active primary 切换及自动回切仍未声明完成。

本地 `cmd/tools/realtime-authority` CLI 提供受约束 writer。Redis Lua CAS 要求非 bootstrap 操作携带当前 raw lease SHA-256，只允许 absent 到 Go epoch 1、active 到下一 epoch frozen、frozen 同 epoch激活目标 authority，以及同状态延长租期；active 之间不能直切。脚本同时写入绝对过期 lease 和七天低敏幂等 receipt，transition ID 重放只有 canonical request hash 一致时成功。CLI 要求显式 `-confirm`、operator/reason，以及首次执行时距当前 5 秒至 1 小时的固定绝对 deadline；重试必须复用同一 deadline，避免 request hash 漂移。reason 只以摘要进入 receipt。当前 operator 是依赖 OS/Redis 权限的审计标签，节点观测和持久 checkpoint receipt 尚未完成，因此 writer 仍不进入 tracked 自动切流。

首轮跨进程恢复证据位于 `benchmarks/c2-cpp-node-delivery-2026-08-28/`。归档候选在 Gateway 不可用时保留 offset，恢复并重启 worker 后重放成功；将已提交 offset 回拨后，Gateway 对稳定 batch 返回 `duplicate=true`，最终 lag 为 0且客户端写入为 0。后续 runner 已改为在同进程有界退避并重试 pending record。

同 workload 对照证据位于 `benchmarks/c2-cpp-comparison-2026-08-28/`。Go baseline 在 20 用户、每用户 2 条文本消息下完成 40/40 accepted、persisted、received；C++ v3 evidence 观察 80 个 Kafka 坐标，`message_type=0` 选择 40 条 workload，40 条好友初始化系统消息保持可见并计为 filtered-out。选中记录全部 projected，node transport requested/observed 为 40/40，最终拒绝和背压为 0，comparison v1 决策为 `eligible`。真实 Go TCP queue saturation race 测试确认容量满时返回 `BACKPRESSURED/QUEUE_FULL`。演练同时发现 Gateway assignment 未进入 readiness，已记录为 `AD-039`；成功候选在负载前显式要求 direct-created 六个分区完成 assignment。

`contracts/realtime-delivery-comparison/v1/report.schema.json` 固定同 workload 对照报告。`scripts/realtime_delivery_comparison.py` 先按 Kafka topic/partition/offset 折叠全部 v3 evidence，再使用 Go baseline 声明的 `message_type` 选择工作负载；初始化系统事件继续计入 observed/filtered-out，避免从原始证据中隐去。选中记录允许 deferred attempt，但要求每个坐标最终 projected、全部请求节点被 observed、最终拒绝/背压为零，并与 Go baseline v4 的 accepted/persisted/received/lag 精确比较。报告只保存双端完整 revision、输入 SHA-256 和聚合计数；结构无效、blocked 和 eligible 分别使用退出码 1、2、0。Harness 单测不能替代真实候选证据。

现有 Go consumer 在 handler 成功返回后提交 Kafka offset，但 Redis `PUBLISH` 和本地 `Client.Enqueue` 没有持久 ACK。v1 legacy adapter 只将当前返回值映射为 `ENQUEUED/OFFLINE`，不改变该语义。

C++ 候选运行时遵守以下门禁：

1. 使用独立 consumer group，只生成 route/batch/ACK 对比证据，不向客户端投递。
2. 对比 source event、目标节点、收件人、connection、mode、ordering key 和处理时延。
3. 有界队列满时返回 `BACKPRESSURED`，不得静默丢弃；显式 primary runner 已保留 offset，真实 saturation 仍需归档。
4. primary 提交 offset 前必须收到完整 terminal 节点 ACK 或证明全部 Presence offline；consume-to-ACK 和进程崩溃重放演练通过后才能评审切流。
5. `OFFLINE` 可提交，消息事实与 Inbox 已持久化；客户端重连后从 Sync Timeline 恢复。

当前主机具备 nlohmann/json 3.11.3；librdkafka 2.3.0 通过 Ubuntu Noble 包无特权解压到临时 sysroot 完成编译与测试，未修改系统包。发布构建必须把 librdkafka 版本写入构建镜像和运行证据，禁止依赖开发机隐式库。

2026-08-28 的首轮真实 Kafka 证据位于 `benchmarks/c2-cpp-shadow-2026-08-28/`：205 条 retained group event 与 1 条注入 poison event 均形成 evidence，最终 lag 为 0；两实例 rebalance 和单实例接管期间 readiness 保持 200。direct topic 当时为空，节点路由和性能对照仍未完成。

## 进程与数据所有权

C++ Delivery 可读取 Kafka 事件与 Redis Presence/热点状态，可向 Gateway 节点发送批次并输出指标。它不得连接 MySQL、Cassandra、Elasticsearch、Agent Runtime 或对象存储，也不得重新执行成员权限、消息持久化和 Conversation/Inbox 投影。

Gateway 继续拥有连接认证、心跳、WebSocket envelope、连接级有界队列和客户端写入。C++ Delivery 达到 shadow 门禁后再评估节点级传输；Gateway 替换属于后续独立里程碑。

## 回滚

`realtime.delivery=go|shadow|cpp` 已进入配置，默认和 tracked Compose 行为保持 `go`。`shadow` 保留 Go 客户端写入并允许 C++ 观察；`cpp` 让 Go message-created group 只前移兼容回滚 checkpoint，并要求 Gateway primary RPC 与 C++ primary 本地配置一致。该开关尚未形成集群共享 fencing，不能单独作为灰度控制面。后续切换必须记录双 group 高水位、稳定窗口和可执行回切 receipt；任何 ACK 漂移、队列溢出、顺序差异或恢复退化都停止候选并恢复 Go 权威链路。

## Compose profile

`deploy/compose/docker-compose.microservices.yml` 默认继续使用 Go authority，`realtime-cpp` profile 默认不创建 C++ 服务。启用 profile 只提供独立的 C++ primary 进程；操作员仍需同时向 Gateway 和 profile 提供 `DIPOLE_REALTIME_DELIVERY=cpp`、`DIPOLE_DELIVERY_PRIMARY_ENABLED=true`、fencing epoch 以及维护窗口决策。profile 使用 Kafka primary group、Redis authority fencing 和 Gateway mTLS node transport，缺少一致切换配置时保持 fail closed。回滚时移除 profile，恢复 `DIPOLE_REALTIME_DELIVERY=go` 与 primary RPC disabled；共享环境切换仍以 C3 checkpoint 和回切 receipt 为准。
